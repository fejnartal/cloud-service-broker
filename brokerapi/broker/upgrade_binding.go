package broker

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/cloud-service-broker/internal/paramparser"
	"github.com/cloudfoundry/cloud-service-broker/utils/correlation"
	"github.com/cloudfoundry/cloud-service-broker/utils/request"
	"github.com/pivotal-cf/brokerapi/v8/domain"
)

func (broker *ServiceBroker) upgradeBindingTF(ctx context.Context, instanceID string, details domain.UpdateDetails) error {
	broker.Logger.Info("Upgrading binding TF", correlation.ID(ctx), lager.Data{
		"instance_id": instanceID,
		"details":     details,
	})

	serviceDefinition, serviceProvider, err := broker.getDefinitionAndProvider(details.ServiceID)
	if err != nil {
		return err
	}

	plan, err := serviceDefinition.GetPlanById(details.PlanID)
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

			storedParams, err := broker.store.GetBindRequestDetails(binding.BindingId, instanceID)
			if err != nil {
				return fmt.Errorf("error retrieving bind request details for %q: %w", instanceID, err)
			}

			parsedDetails := paramparser.BindDetails{
				PlanID:        details.PlanID,
				ServiceID:     details.ServiceID,
				RequestParams: storedParams,
			}

			vars, err := serviceDefinition.BindVariables(instance, binding.BindingId, parsedDetails, plan, request.DecodeOriginatingIdentityHeader(ctx))
			if err != nil {
				return err
			}

			if err := serviceProvider.UpgradeBinding(ctx, instanceID, binding.BindingId, vars); err != nil {
				return err
			}
		}
	}

	return nil
}
