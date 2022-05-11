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

package broker

import (
	"context"
	"fmt"

	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/workspace"

	"github.com/cloudfoundry/cloud-service-broker/db_service/models"
	"github.com/cloudfoundry/cloud-service-broker/internal/storage"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/cloud-service-broker/internal/paramparser"
	"github.com/cloudfoundry/cloud-service-broker/utils/correlation"
	"github.com/cloudfoundry/cloud-service-broker/utils/request"
	"github.com/pivotal-cf/brokerapi/v8/domain"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"
)

// Provision creates a new instance of a service.
// It is bound to the `PUT /v2/service_instances/:instance_id` endpoint and can be called using the `cf create-service` command.
func (broker *ServiceBroker) Provision(ctx context.Context, instanceID string, details domain.ProvisionDetails, clientSupportsAsync bool) (domain.ProvisionedServiceSpec, error) {
	broker.Logger.Info("Provisioning", correlation.ID(ctx), lager.Data{
		"instanceId":         instanceID,
		"accepts_incomplete": clientSupportsAsync,
		"details":            details,
	})

	// make sure that instance hasn't already been provisioned
	exists, err := broker.store.ExistsServiceInstanceDetails(instanceID)
	switch {
	case err != nil:
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("database error checking for existing instance: %s", err)
	case exists:
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrInstanceAlreadyExists
	}

	parsedDetails, err := paramparser.ParseProvisionDetails(details)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, ErrInvalidUserInput
	}

	brokerService, serviceHelper, err := broker.getDefinitionAndProvider(parsedDetails.ServiceID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	// verify the service exists and the plan exists
	plan, err := brokerService.GetPlanByID(parsedDetails.PlanID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	// verify async provisioning is allowed if it is required
	shouldProvisionAsync := serviceHelper.ProvisionsAsync()
	if shouldProvisionAsync && !clientSupportsAsync {
		return domain.ProvisionedServiceSpec{}, apiresponses.ErrAsyncRequired
	}

	// Give the user a better error message if they give us a bad request
	if err := validateProvisionParameters(
		parsedDetails.RequestParams,
		brokerService.ProvisionInputVariables,
		brokerService.ImportInputVariables,
		plan); err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	// validate parameters meet the service's schema and merge the user vars with
	// the plan's
	vars, err := brokerService.ProvisionVariables(instanceID, parsedDetails, *plan, request.DecodeOriginatingIdentityHeader(ctx))
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}

	// get instance details
	instanceDetails := storage.ServiceInstanceDetails{
		OperationGUID: vars.GetString("tf_id"),
		OperationType: models.ProvisionOperationType,
	}

	// save instance details
	instanceDetails.ServiceGUID = parsedDetails.ServiceID
	instanceDetails.GUID = instanceID
	instanceDetails.PlanGUID = parsedDetails.PlanID
	instanceDetails.SpaceGUID = parsedDetails.SpaceGUID
	instanceDetails.OrganizationGUID = parsedDetails.OrganizationGUID

	if err := broker.store.StoreServiceInstanceDetails(instanceDetails); err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("error saving instance details to database: %s. WARNING: this instance cannot be deprovisioned through cf. Contact your operator for cleanup", err)
	}

	// save provision request details
	if err := broker.store.StoreProvisionRequestDetails(instanceID, parsedDetails.RequestParams); err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("error saving provision request details to database: %s. Services relying on async provisioning will not be able to complete provisioning", err)
	}

	deployment, err := broker.store.GetTerraformDeployment(instanceDetails.OperationGUID)
	if err != nil {
		return domain.ProvisionedServiceSpec{}, err
	}
	if err := broker.markJobStarted(deployment, models.ProvisionOperationType); err != nil {
		return domain.ProvisionedServiceSpec{}, fmt.Errorf("error marking job started: %w", err)
	}

	// Do things
	go func() {

		serviceHelper.Provision(ctx, vars)
		broker.operationFinished(err, workspace, deployment)

	}()

	return domain.ProvisionedServiceSpec{IsAsync: shouldProvisionAsync, DashboardURL: "", OperationData: instanceDetails.OperationGUID}, nil
}

func (broker *ServiceBroker) markJobStarted(deployment storage.TerraformDeployment, operationType string) error {
	// update the deployment info
	deployment.LastOperationType = operationType
	deployment.LastOperationState = InProgress
	deployment.LastOperationMessage = ""

	if err := broker.store.StoreTerraformDeployment(deployment); err != nil {
		return err
	}

	return nil
}

func (runner *ServiceBroker) operationFinished(err error, workspace workspace.Workspace, deployment storage.TerraformDeployment) error {
	// we shouldn't update the status on update when updating the HCL, as the status comes either from the provision call or a previous update
	if err == nil {
		lastOperationMessage := ""
		// maybe do if deployment.LastOperationType != "validation" so we don't do the status update on staging a job.
		// previously we would only stage a job on provision so state would be empty and the outputs would be null.
		outputs, err := workspace.Outputs(workspace.ModuleInstances()[0].InstanceName)
		if err == nil {
			if status, ok := outputs["status"]; ok {
				lastOperationMessage = fmt.Sprintf("%v", status)
			}
		}
		deployment.LastOperationState = Succeeded
		deployment.LastOperationMessage = deployment.GetStatusFromWorkspace()
	} else {
		deployment.LastOperationState = Failed
		deployment.LastOperationMessage = err.Error()
	}

	if err != nil {
		deployment.LastOperationState = Failed
		deployment.LastOperationMessage = fmt.Sprintf("couldn't serialize workspace, contact your operator for cleanup: %s", err.Error())
	}

	return runner.store.StoreTerraformDeployment(deployment)
}

const (
	InProgress = "in progress"
	Succeeded  = "succeeded"
	Failed     = "failed"
)
