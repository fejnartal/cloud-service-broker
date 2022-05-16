// Copyright 2018 the Service Broker Project Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tf

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/executor"
	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/invoker"

	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/workspace"

	"github.com/hashicorp/go-version"

	"github.com/spf13/viper"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/cloud-service-broker/pkg/broker"
	"github.com/cloudfoundry/cloud-service-broker/utils"
	"github.com/cloudfoundry/cloud-service-broker/utils/correlation"
)

const (
	InProgress = "in progress"
	Succeeded  = "succeeded"
	Failed     = "failed"

	TfUpgradeEnabled = "brokerpak.terraform.upgrades.enabled"
)

func init() {
	viper.BindEnv(TfUpgradeEnabled, "TERRAFORM_UPGRADES_ENABLED")
	viper.SetDefault(TfUpgradeEnabled, false)
}

// NewTfJobRunner constructs a new JobRunner for the given project.
func NewTfJobRunner(store broker.ServiceProviderStorage,
	tfBinContext executor.TFBinariesContext,
	workspaceFactory workspace.WorkspaceBuilder,
	invokerBuilder invoker.TerraformInvokerBuilder) *TfJobRunner {
	return &TfJobRunner{
		store:                   store,
		tfBinContext:            tfBinContext,
		WorkspaceBuilder:        workspaceFactory,
		TerraformInvokerBuilder: invokerBuilder,
	}
}

// TfJobRunner is responsible for executing terraform jobs in the background and
// providing a way to log and access the state of those background tasks.
//
// Jobs are given an ID and a workspace to operate in, and then the TfJobRunner
// is told which Terraform commands to execute on the given job.
// The TfJobRunner executes those commands in the background and keeps track of
// their state in a database table which gets updated once the task is completed.
//
// The TfJobRunner keeps track of the workspace and the Terraform state file so
// subsequent commands will operate on the same structure.
type TfJobRunner struct {
	// executor holds a custom executor that will be called when commands are run.
	store        broker.ServiceProviderStorage
	tfBinContext executor.TFBinariesContext
	workspace.WorkspaceBuilder
	invoker.TerraformInvokerBuilder
}

// ImportResource represents TF resource to IaaS resource ID mapping for import
type ImportResource struct {
	TfResource   string
	IaaSResource string
}

// Import runs `terraform import` and `terraform apply` on the given workspace in the background.
// The status of the job can be found by polling the Status function.
func (runner *TfJobRunner) Import(ctx context.Context, id string, importResources []ImportResource) (error, workspace.Workspace) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return err, nil
	}

	workspace, err := workspace.DeserializeWorkspace(deployment.Workspace)
	if err != nil {
		return err, nil
	}

	invoker := runner.DefaultInvoker()

	logger := utils.NewLogger("Import").WithData(correlation.ID(ctx))
	resources := make(map[string]string)
	for _, resource := range importResources {
		resources[resource.TfResource] = resource.IaaSResource
	}

	if err := invoker.Import(ctx, workspace, resources); err != nil {
		logger.Error("Import Failed", err)
		return err, workspace
	}
	mainTf, err := invoker.Show(ctx, workspace)
	if err == nil {
		var tf string
		var parameterVals map[string]string
		tf, parameterVals, err = workspace.Transformer.ReplaceParametersInTf(workspace.Transformer.AddParametersInTf(workspace.Transformer.CleanTf(mainTf)))
		if err == nil {
			for pn, pv := range parameterVals {
				workspace.Instances[0].Configuration[pn] = pv
			}
			workspace.Modules[0].Definitions["main"] = tf

			logger.Info("new workspace", lager.Data{
				"workspace": workspace,
				"tf":        tf,
			})

			err = runner.terraformPlanToCheckNoResourcesDeleted(invoker, ctx, workspace, logger)
			if err != nil {
				logger.Error("plan failed", err)
			} else {
				err = invoker.Apply(ctx, workspace)
			}
		}
	}
	return err, workspace

}

func (runner *TfJobRunner) terraformPlanToCheckNoResourcesDeleted(invoker invoker.TerraformInvoker, ctx context.Context, workspace *workspace.TerraformWorkspace, logger lager.Logger) error {
	planOutput, err := invoker.Plan(ctx, workspace)
	if err != nil {
		return err
	}
	err = CheckTerraformPlanOutput(logger, planOutput)
	return err
}

func (runner *TfJobRunner) DefaultInvoker() invoker.TerraformInvoker {
	return runner.VersionedInvoker(runner.tfBinContext.DefaultTfVersion)
}

func (runner *TfJobRunner) VersionedInvoker(version *version.Version) invoker.TerraformInvoker {
	return runner.VersionedTerraformInvoker(version)
}

// Create runs `terraform apply` on the given workspace in the background.
// The status of the job can be found by polling the Status function.
func (runner *TfJobRunner) Create(ctx context.Context, id string) (error, workspace.Workspace) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return fmt.Errorf("error getting TF deployment: %w", err), nil
	}

	workspace, err := runner.CreateWorkspace(deployment)
	if err != nil {
		return fmt.Errorf("error hydrating workspace: %w", err), nil
	}

	return runner.DefaultInvoker().Apply(ctx, workspace), workspace
}

func (runner *TfJobRunner) Update(ctx context.Context, id string, templateVars map[string]interface{}) (error, workspace.Workspace) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return err, nil
	}

	workspace, err := runner.CreateWorkspace(deployment)
	if err != nil {
		return err, nil
	}

	err = runner.performTerraformUpgrade(ctx, workspace)
	if err != nil {
		return err, workspace
	}

	err = workspace.UpdateInstanceConfiguration(templateVars)
	if err != nil {
		return err, workspace
	}

	err = runner.DefaultInvoker().Apply(ctx, workspace)

	return err, workspace
}

func (runner *TfJobRunner) performTerraformUpgrade(ctx context.Context, workspace workspace.Workspace) error {
	currentTfVersion, err := workspace.StateVersion()
	if err != nil {
		return err
	}

	if viper.GetBool(TfUpgradeEnabled) {
		if currentTfVersion.LessThan(runner.tfBinContext.DefaultTfVersion) {
			if runner.tfBinContext.TfUpgradePath == nil || len(runner.tfBinContext.TfUpgradePath) == 0 {
				return errors.New("terraform version mismatch and no upgrade path specified")
			}
			for _, targetTfVersion := range runner.tfBinContext.TfUpgradePath {
				if currentTfVersion.LessThan(targetTfVersion) {
					err = runner.VersionedInvoker(targetTfVersion).Apply(ctx, workspace)
					if err != nil {
						return err
					}
				}
			}
		}
	} else if currentTfVersion.LessThan(runner.tfBinContext.DefaultTfVersion) {
		return errors.New("apply attempted with a newer version of terraform than the state")
	}

	return nil
}

// Destroy runs `terraform destroy` on the given workspace in the background.
// The status of the job can be found by polling the Status function.
func (runner *TfJobRunner) Destroy(ctx context.Context, id string, templateVars map[string]interface{}) (error, workspace.Workspace) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return err, nil
	}

	workspace, err := workspace.DeserializeWorkspace(deployment.Workspace)
	if err != nil {
		return err, nil
	}

	inputList, err := workspace.Modules[0].Inputs()
	if err != nil {
		return err, nil
	}

	limitedConfig := make(map[string]interface{})
	for _, name := range inputList {
		limitedConfig[name] = templateVars[name]
	}

	workspace.Instances[0].Configuration = limitedConfig

	err = runner.performTerraformUpgrade(ctx, workspace)
	if err != nil {
		return err, workspace
	}

	err = runner.DefaultInvoker().Destroy(ctx, workspace)
	return err, workspace
}

// Outputs gets the output variables for the given module instance in the workspace.
func (runner *TfJobRunner) Outputs(ctx context.Context, id, instanceName string) (map[string]interface{}, error) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return nil, fmt.Errorf("error getting TF deployment: %w", err)
	}

	ws, err := workspace.DeserializeWorkspace(deployment.Workspace)
	if err != nil {
		return nil, fmt.Errorf("error deserializing workspace: %w", err)
	}

	return ws.Outputs(instanceName)
}

// Show returns the output from terraform show command
func (runner *TfJobRunner) Show(ctx context.Context, id string) (string, error) {
	deployment, err := runner.store.GetTerraformDeployment(id)
	if err != nil {
		return "", err
	}

	workspace, err := workspace.DeserializeWorkspace(deployment.Workspace)
	if err != nil {
		return "", err
	}

	return runner.DefaultInvoker().Show(ctx, workspace)
}
