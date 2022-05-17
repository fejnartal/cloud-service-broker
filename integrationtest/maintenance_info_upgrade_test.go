package integrationtest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/onsi/gomega/gbytes"

	"github.com/cloudfoundry/cloud-service-broker/db_service/models"

	"github.com/cloudfoundry/cloud-service-broker/integrationtest/helper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/brokerapi/v8/domain"
)

var _ = Describe("Terraform binding upgrade on instance update", func() {
	const serviceOfferingGUID = "df2c1512-3013-11ec-8704-2fbfa9c8a802"
	const servicePlanGUID = "e59773ce-3013-11ec-9bbb-9376b4f72d14"

	var (
		testHelper     *helper.TestHelper
		session        *Session
		mi             *domain.MaintenanceInfo
		miLower        *domain.MaintenanceInfo
		previousValues domain.PreviousValues
	)

	BeforeEach(func() {
		testHelper = helper.New(csb)
		testHelper.BuildBrokerpak(testHelper.OriginalDir, "fixtures", "brokerpak-terraform-0.12")

		session = testHelper.StartBroker()
	})

	AfterEach(func() {
		session.Terminate()
		testHelper.Restore()
	})

	terraformStateVersion := func(serviceInstanceGUID string) string {
		var tfDeploymentReceiver models.TerraformDeployment
		Expect(testHelper.DBConn().Where("id = ?", fmt.Sprintf("tf:%s:", serviceInstanceGUID)).First(&tfDeploymentReceiver).Error).NotTo(HaveOccurred())
		var workspaceReceiver struct {
			State []byte `json:"tfstate"`
		}
		Expect(json.Unmarshal(tfDeploymentReceiver.Workspace, &workspaceReceiver)).NotTo(HaveOccurred())
		var stateReceiver struct {
			Version string `json:"terraform_version"`
		}
		Expect(json.Unmarshal(workspaceReceiver.State, &stateReceiver)).NotTo(HaveOccurred())
		return stateReceiver.Version
	}

	bindingTerraformStateVersion := func(serviceInstanceGUID, bindingGUID string) string {
		var tfDeploymentReceiver models.TerraformDeployment
		Expect(testHelper.DBConn().Where("id = ?", fmt.Sprintf("tf:%s:%s", serviceInstanceGUID, bindingGUID)).First(&tfDeploymentReceiver).Error).NotTo(HaveOccurred())
		var workspaceReceiver struct {
			State []byte `json:"tfstate"`
		}
		Expect(json.Unmarshal(tfDeploymentReceiver.Workspace, &workspaceReceiver)).NotTo(HaveOccurred())
		var stateReceiver struct {
			Version string `json:"terraform_version"`
		}
		Expect(json.Unmarshal(workspaceReceiver.State, &stateReceiver)).NotTo(HaveOccurred())
		return stateReceiver.Version
	}

	Context("TF Upgrades are enabled", func() {
		BeforeEach(func() {
			mi = &domain.MaintenanceInfo{Version: "0.2.0"}

			miLower = &domain.MaintenanceInfo{Version: "0.1.0"}

			previousValues = domain.PreviousValues{
				PlanID:          servicePlanGUID,
				ServiceID:       serviceOfferingGUID,
				OrgID:           "",
				SpaceID:         "",
				MaintenanceInfo: miLower,
			}
		})

		Describe("an upgrade is available", func() {
			When("upgrade service is invoked", func() {
				It("upgrades tf state version to latest for instance and binding", func() {
					By("provisioning a service instance at 0.12")
					serviceInstanceGUID := uuid.New()
					provisionResponse := testHelper.Client().Provision(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
					Expect(provisionResponse.Error).NotTo(HaveOccurred())
					Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))

					Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
					Expect(pollLastOperation(testHelper, serviceInstanceGUID)()).To(Equal(domain.Succeeded))
					Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("0.12.21"))

					By("creating a binding")
					bindGUID := uuid.New()
					bindResponse := testHelper.Client().Bind(serviceInstanceGUID, bindGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)

					Expect(bindResponse.Error).NotTo(HaveOccurred())
					Expect(bindResponse.StatusCode).To(Equal(http.StatusCreated))

					By("updating the brokerpak and restarting the broker")
					session.Terminate()
					testHelper.BuildBrokerpak(testHelper.OriginalDir, "fixtures", "brokerpak-terraform-upgrade-mi")

					session = testHelper.StartBroker("TERRAFORM_UPGRADES_ENABLED=true")

					By("running 'cf update-service'")
					updateResponse := testHelper.Client().Upgrade(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil, mi, previousValues)
					Expect(updateResponse.Error).NotTo(HaveOccurred())
					Expect(updateResponse.StatusCode).To(Equal(http.StatusAccepted))

					Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
					Expect(pollLastOperation(testHelper, serviceInstanceGUID)()).To(Equal(domain.Succeeded))

					By("observing that the instance TF state file has been updated to the latest version")
					Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("1.1.6"))

					By("observing that the binding TF state file has been updated to the latest version")
					Expect(bindingTerraformStateVersion(serviceInstanceGUID, bindGUID)).To(Equal("1.1.6"))

				})
			})

			When("a service instance is updated with 20 bindings", func() {
				It("upgrades tf state version to latest for instance and bindings", func() {
					By("provisioning a service instance at 0.12")
					serviceInstanceGUID := uuid.New()
					provisionResponse := testHelper.Client().Provision(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
					Expect(provisionResponse.Error).NotTo(HaveOccurred())
					Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))

					Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
					Expect(pollLastOperation(testHelper, serviceInstanceGUID)()).To(Equal(domain.Succeeded))
					Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("0.12.21"))

					By("creating 20 bindings")
					var bindGUIDs []string
					for i := 0; i < 20; i++ {
						bindGUID := uuid.New()
						bindResponse := testHelper.Client().Bind(serviceInstanceGUID, bindGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
						Expect(bindResponse.Error).NotTo(HaveOccurred())
						Expect(bindResponse.StatusCode).To(Equal(http.StatusCreated))

						bindGUIDs = append(bindGUIDs, bindGUID)
					}

					By("updating the brokerpak and restarting the broker")
					session.Terminate()
					testHelper.BuildBrokerpak(testHelper.OriginalDir, "fixtures", "brokerpak-terraform-upgrade-mi")

					session = testHelper.StartBroker("TERRAFORM_UPGRADES_ENABLED=true")

					By("running 'cf update-service'")
					updateResponse := testHelper.Client().Upgrade(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil, mi, previousValues)
					Expect(updateResponse.Error).NotTo(HaveOccurred())
					Expect(updateResponse.StatusCode).To(Equal(http.StatusAccepted))

					Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
					Expect(pollLastOperation(testHelper, serviceInstanceGUID)()).To(Equal(domain.Succeeded))

					By("observing that the instance TF state file has been updated to the latest version")
					Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("1.1.6"))

					By("observing that the binding TF state file has been updated to the latest version")
					for _, guid := range bindGUIDs {
						Expect(bindingTerraformStateVersion(serviceInstanceGUID, guid)).To(Equal("1.1.6"))
					}

				})
			})

			When("a binding is requested", func() {
				It("should fail to create binding ", func() {
					By("provisioning a service instance at 0.12")
					serviceInstanceGUID := uuid.New()
					provisionResponse := testHelper.Client().Provision(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
					Expect(provisionResponse.Error).NotTo(HaveOccurred())
					Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))

					Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
					Expect(pollLastOperation(testHelper, serviceInstanceGUID)()).To(Equal(domain.Succeeded))
					Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("0.12.21"))

					By("updating the brokerpak and restarting the broker")
					session.Terminate()
					testHelper.BuildBrokerpak(testHelper.OriginalDir, "fixtures", "brokerpak-terraform-upgrade-mi")

					session = testHelper.StartBroker("TERRAFORM_UPGRADES_ENABLED=true")

					By("creating a binding")
					bindGUID := uuid.New()
					bindResponse := testHelper.Client().Bind(serviceInstanceGUID, bindGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)

					Expect(bindResponse.Error).NotTo(HaveOccurred())
					Expect(bindResponse.StatusCode).To(Equal(http.StatusInternalServerError))

					By("observing that the destroy failed due to mismatched TF versions")
					Eventually(session).WithTimeout(10 * time.Second).Should(gbytes.Say("an upgrade is available for this instance"))

				})
			})
		})

	})

	Context("TF Upgrades are disabled", func() {
		When("a service instance is updated", func() {
			It("does not upgrade the instance and binding tf", func() {
				By("provisioning a service instance at 0.12")
				serviceInstanceGUID := uuid.New()
				provisionResponse := testHelper.Client().Provision(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
				Expect(provisionResponse.Error).NotTo(HaveOccurred())
				Expect(provisionResponse.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).Should(Equal(domain.Succeeded))
				Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("0.12.21"))

				By("creating a binding")
				bindGUID := uuid.New()
				bindResponse := testHelper.Client().Bind(serviceInstanceGUID, bindGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)

				Expect(bindResponse.Error).NotTo(HaveOccurred())
				Expect(bindResponse.StatusCode).To(Equal(http.StatusCreated))

				By("updating the brokerpak and restarting the broker")
				session.Terminate()
				testHelper.BuildBrokerpak(testHelper.OriginalDir, "fixtures", "brokerpak-terraform-upgrade")
				session = testHelper.StartBroker()

				By("running 'cf update-service'")
				testHelper.Client().Update(serviceInstanceGUID, serviceOfferingGUID, servicePlanGUID, requestID(), nil)
				//Expect(updateResponse.StatusCode).To(Equal(http.StatusInternalServerError))

				Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).ShouldNot(Equal(domain.InProgress))
				Eventually(pollLastOperation(testHelper, serviceInstanceGUID), time.Minute*2, lastOperationPollingFrequency).Should(Equal(domain.Failed))

				By("observing that the instance TF state file has been updated to the latest version")
				Expect(terraformStateVersion(serviceInstanceGUID)).To(Equal("0.12.21"))

				By("observing that the binding TF state file has been updated to the latest version")
				Expect(bindingTerraformStateVersion(serviceInstanceGUID, bindGUID)).To(Equal("0.12.21"))
			})
		})
	})
})
