package decider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/pivotal-cf/brokerapi/v8/domain/apiresponses"

	"github.com/cloudfoundry/cloud-service-broker/pkg/broker"
	"github.com/pivotal-cf/brokerapi/v8/domain"
)

type Operation int

const (
	Update Operation = iota
	Upgrade
	Failed
)

var errInstanceMustBeUpgradedFirst = apiresponses.NewFailureResponseBuilder(
	errors.New("service instance needs to be upgraded before updating"),
	http.StatusUnprocessableEntity,
	"previous-maintenance-info-check",
).Build()

var warningMaintenanceInfoNilInTheRequest = errors.New(
	"maintenance info defined in broker service catalog, but not passed in request",
)

func DecideOperation(service *broker.ServiceDefinition, details domain.UpdateDetails) (Operation, error) {
	// Validate maintenance info in request
	// If plan not changed, request params empty, maintenance info values differ
	//   return Upgrade
	// return Update

	if err := validateMaintenanceInfo(service, details.PlanID, details.MaintenanceInfo); err != nil {
		if err == warningMaintenanceInfoNilInTheRequest {
			if err = validatePreviousMaintenanceInfo(details, service); err == nil {
				return Update, nil
			}
		}
		return Failed, err
	}

	if planNotChanged(details) && requestParamsEmpty(details) && requestMaintenanceInfoValuesDiffer(details) {
		return Upgrade, nil
	}

	if err := validatePreviousMaintenanceInfo(details, service); err != nil {
		return Failed, err
	}

	return Update, nil
}

func planNotChanged(details domain.UpdateDetails) bool {
	return details.PlanID == details.PreviousValues.PlanID
}

func requestParamsEmpty(details domain.UpdateDetails) bool {
	if len(details.RawParameters) == 0 {
		return true
	}

	var params map[string]interface{}
	if err := json.Unmarshal(details.RawParameters, &params); err != nil {
		return false
	}
	return len(params) == 0
}

func requestMaintenanceInfoValuesDiffer(details domain.UpdateDetails) bool {
	if details.MaintenanceInfo == nil && details.PreviousValues.MaintenanceInfo != nil {
		return true
	}

	if details.MaintenanceInfo != nil && details.PreviousValues.MaintenanceInfo == nil {
		return true
	}

	if details.MaintenanceInfo == nil && details.PreviousValues.MaintenanceInfo == nil {
		return false
	}

	return !details.MaintenanceInfo.Equals(*details.PreviousValues.MaintenanceInfo)
}

func validateMaintenanceInfo(service *broker.ServiceDefinition, planID string, maintenanceInfo *domain.MaintenanceInfo) error {
	planMaintenanceInfo, err := getMaintenanceInfoForPlan(planID, service)
	if err != nil {
		return err
	}

	if maintenanceInfoConflict(maintenanceInfo, planMaintenanceInfo) {
		if maintenanceInfo == nil {
			return warningMaintenanceInfoNilInTheRequest
		}

		if planMaintenanceInfo == nil {
			return apiresponses.ErrMaintenanceInfoNilConflict
		}

		return apiresponses.ErrMaintenanceInfoConflict
	}

	return nil
}

func validatePreviousMaintenanceInfo(details domain.UpdateDetails, service *broker.ServiceDefinition) error {
	if details.PreviousValues.MaintenanceInfo != nil {
		if previousPlanMaintenanceInfo, err := getMaintenanceInfoForPlan(details.PreviousValues.PlanID, service); err == nil {
			if maintenanceInfoConflict(details.PreviousValues.MaintenanceInfo, previousPlanMaintenanceInfo) {
				return errInstanceMustBeUpgradedFirst
			}
		}
	}
	return nil
}

func getMaintenanceInfoForPlan(id string, service *broker.ServiceDefinition) (*domain.MaintenanceInfo, error) {
	for _, plan := range service.Plans {
		if plan.ID == id {
			return &domain.MaintenanceInfo{
				Version:     plan.MaintenanceInfo.Version,
				Description: plan.MaintenanceInfo.Description,
			}, nil
		}
	}

	return nil, fmt.Errorf("plan %s does not exist", id)
}

func maintenanceInfoConflict(a, b *domain.MaintenanceInfo) bool {
	if a != nil && b != nil {
		return !a.Equals(*b)
	}

	if a == nil && b == nil {
		return false
	}

	return true
}
