package tf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudfoundry/cloud-service-broker/internal/storage"
	"github.com/cloudfoundry/cloud-service-broker/pkg/broker"
	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/workspace"
)

type LastOperationStoreImpl struct {
	store broker.ServiceProviderStorage
}

func NewLastOperationStore(store broker.ServiceProviderStorage) LastOperationStore {
	return &LastOperationStoreImpl{
		store: store,
	}
}

func (lastOperationStore *LastOperationStoreImpl) MarkJobStarted(deployment storage.TerraformDeployment, operationType string) error {
	// update the deployment info
	deployment.LastOperationType = operationType
	deployment.LastOperationState = InProgress
	deployment.LastOperationMessage = ""

	if err := lastOperationStore.store.StoreTerraformDeployment(deployment); err != nil {
		return err
	}

	return nil
}

func (lastOperationStore *LastOperationStoreImpl) MarkJobStartedNew(id string, operationType string) error {
	deployment, err := lastOperationStore.store.GetTerraformDeployment(id)
	if err != nil {
		return fmt.Errorf("error getting TF deployment: %w", err)
	}
	deployment.LastOperationType = operationType
	deployment.LastOperationState = InProgress
	deployment.LastOperationMessage = ""

	if err := lastOperationStore.store.StoreTerraformDeployment(deployment); err != nil {
		return err
	}

	return nil
}

// OperationFinished closes out the state of the background job so clients that
// are polling can get the results.
func (lastOperationStore *LastOperationStoreImpl) OperationFinished(err error, workspace workspace.Workspace, deployment storage.TerraformDeployment) error {
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
		deployment.LastOperationMessage = lastOperationMessage
	} else {
		deployment.LastOperationState = Failed
		deployment.LastOperationMessage = err.Error()
	}

	workspaceString, err := workspace.Serialize()
	if err != nil {
		deployment.LastOperationState = Failed
		deployment.LastOperationMessage = fmt.Sprintf("couldn't serialize workspace, contact your operator for cleanup: %s", err.Error())
	}

	deployment.Workspace = []byte(workspaceString)

	return lastOperationStore.store.StoreTerraformDeployment(deployment)
}

// Wait waits for an operation to complete, polling its status once per second.
func (lastOperationStore *LastOperationStoreImpl) Wait(ctx context.Context, id string) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-time.After(1 * time.Second):
			isDone, _, err := lastOperationStore.Status(ctx, id)
			if isDone {
				return err
			}
		}
	}
}

// Status gets the status of the most recent job on the workspace.
// If isDone is true, then the status of the operation will not change again.
// if isDone is false, then the operation is ongoing.
func (lastOperationStore *LastOperationStoreImpl) Status(ctx context.Context, id string) (bool, string, error) {
	deployment, err := lastOperationStore.store.GetTerraformDeployment(id)
	if err != nil {
		return true, "", err
	}

	switch deployment.LastOperationState {
	case Succeeded:
		return true, deployment.LastOperationMessage, nil
	case Failed:
		return true, deployment.LastOperationMessage, errors.New(deployment.LastOperationMessage)
	default:
		return false, deployment.LastOperationMessage, nil
	}
}
