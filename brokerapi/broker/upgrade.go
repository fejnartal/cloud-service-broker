package broker

import (
	"context"
	"fmt"

	"github.com/cloudfoundry/cloud-service-broker/internal/paramparser"
	"github.com/cloudfoundry/cloud-service-broker/utils/request"
	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"

	"github.com/pivotal-cf/brokerapi/v8/domain"
)

func (broker *ServiceBroker) upgrade(ctx context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) error {

	instance, err := broker.store.GetServiceInstanceDetails(instanceID)
	if err != nil {
		return fmt.Errorf("database error getting existing instance: %s", err)
	}

	brokerService, serviceProvider, err := broker.getDefinitionAndProvider(instance.ServiceGUID)
	if err != nil {
		return err
	}

	// verify the service exists and the plan exists
	plan, err := brokerService.GetPlanByID(details.PlanID)
	if err != nil {
		return err
	}

	// verify async provisioning is allowed if it is required
	shouldProvisionAsync := serviceProvider.ProvisionsAsync()
	if shouldProvisionAsync && !asyncAllowed {
		return apiresponses.ErrAsyncRequired
	}

	provisionDetails, err := broker.store.GetProvisionRequestDetails(instanceID)
	if err != nil {
		return fmt.Errorf("error retrieving provision request details for %q: %w", instanceID, err)
	}

	importedParams, err := serviceProvider.GetImportedProperties(ctx, instance.PlanGUID, instance.GUID, brokerService.ProvisionInputVariables)
	if err != nil {
		return fmt.Errorf("error retrieving subsume parameters for %q: %w", instanceID, err)
	}

	mergedDetails, err := mergeJSON(provisionDetails, map[string]interface{}{}, importedParams)
	if err != nil {
		return fmt.Errorf("error merging update and provision details: %w", err)
	}

	vars, err := brokerService.UpdateVariables(instanceID, paramparser.UpdateDetails{}, mergedDetails, *plan, request.DecodeOriginatingIdentityHeader(ctx))
	if err != nil {
		return err
	}

	tfID := vars.GetString("tf_id")

	// Upgrade Instance
	err = serviceProvider.Upgrade(ctx, tfID, vars)
	if err != nil {
		return err
	}

	bindingTFIDs, err := broker.store.GetServiceBindingIDs(instanceID)
	if err != nil {
		return err
	}

	if len(bindingTFIDs) > 0 {
		for _, binding := range bindingTFIDs {
			// get existing service instance details
			instance, err := broker.store.GetServiceInstanceDetails(instanceID)
			if err != nil {
				return fmt.Errorf("error retrieving service instance details: %s", err)
			}

			storedParams, err := broker.store.GetBindRequestDetails(binding.BindingID, instanceID)
			if err != nil {
				return fmt.Errorf("error retrieving bind request details for %q: %w", instanceID, err)
			}

			parsedDetails := paramparser.BindDetails{
				PlanID:        details.PlanID,
				ServiceID:     details.ServiceID,
				RequestParams: storedParams,
			}

			vars, err := brokerService.BindVariables(instance, binding.BindingID, parsedDetails, plan, request.DecodeOriginatingIdentityHeader(ctx))
			if err != nil {
				return err
			}

			tfID = generateTfID(instanceID, binding.BindingID)

			if err := serviceProvider.Upgrade(ctx, tfID, vars); err != nil {
				return err
			}
		}
	}

	return nil

}

func generateTfID(instanceID, bindingID string) string {
	return fmt.Sprintf("tf:%s:%s", instanceID, bindingID)
}
