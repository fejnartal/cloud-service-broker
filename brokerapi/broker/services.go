package broker

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/executor"
	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/invoker"
	"github.com/cloudfoundry/cloud-service-broker/pkg/providers/tf/workspace"

	"github.com/cloudfoundry/cloud-service-broker/pkg/broker"
	"github.com/pivotal-cf/brokerapi/v8/domain"
)

// Services lists services in the broker's catalog.
// It is called through the `GET /v2/catalog` endpoint or the `cf marketplace` command.
func (broker *ServiceBroker) Services(ctx context.Context) ([]domain.Service, error) {
	var svcs []domain.Service

	enabledServices, err := broker.registry.GetEnabledServices()
	if err != nil {
		return nil, err
	}
	for _, service := range enabledServices {
		entry := service.CatalogEntry()
		svcs = append(svcs, entry.ToPlain())
	}

	return svcs, nil
}

func (broker *ServiceBroker) getDefinitionAndProvider(serviceID string) (*broker.ServiceDefinition, broker.ServiceProvider, error) {
	defn, err := broker.registry.GetServiceByID(serviceID)
	if err != nil {
		return nil, nil, err
	}

	serviceProvider := broker.providerBuilder.BuildProvider(defn, broker.store, broker.Logger)
	return defn, serviceProvider, nil
}

func (broker *ServiceBroker) getServiceName(def *broker.ServiceDefinition) string {
	return def.Name
}

func getCredentialName(serviceName, bindingID string) string {
	return fmt.Sprintf("/c/%s/%s/%s/secrets-and-services", credhubClientIdentifier, serviceName, bindingID)
}

type TFProviderBuilder struct{}

func (p TFProviderBuilder) BuildProvider(defn *broker.ServiceDefinition, store broker.ServiceProviderStorage, logger lager.Logger) broker.ServiceProvider {
	executorFactory := executor.NewExecutorFactory(defn.TfBinContext.Dir, defn.TfBinContext.Params, defn.EnvVars)
	return broker.NewTerraformProvider(broker.NewTfJobRunner(store, defn.TfBinContext, workspace.NewWorkspaceFactory(), invoker.NewTerraformInvokerFactory(executorFactory, defn.TfBinContext.Dir, defn.TfBinContext.ProviderReplacements)), logger, defn.ConstDefn, store)
}
