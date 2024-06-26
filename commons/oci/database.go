/*
** Copyright (c) 2022 Oracle and/or its affiliates.
**
** The Universal Permissive License (UPL), Version 1.0
**
** Subject to the condition set forth below, permission is hereby granted to any
** person obtaining a copy of this software, associated documentation and/or data
** (collectively the "Software"), free of charge and under any and all copyright
** rights in the Software, and any and all patent rights owned or freely
** licensable by each licensor hereunder covering either (i) the unmodified
** Software as contributed to or provided by such licensor, or (ii) the Larger
** Works (as defined below), to deal in both
**
** (a) the Software, and
** (b) any piece of software and/or hardware listed in the lrgrwrks.txt file if
** one is included with the Software (each a "Larger Work" to which the Software
** is contributed by such licensors),
**
** without restriction, including without limitation the rights to copy, create
** derivative works of, display, perform, and distribute the Software and make,
** use, sell, offer for sale, import, export, have made, and have sold the
** Software and the Larger Work(s), and to sublicense the foregoing rights on
** either these or other terms.
**
** This license is subject to the following condition:
** The above copyright notice and either this complete permission notice or at
** a minimum a reference to the UPL must be included in all copies or
** substantial portions of the Software.
**
** THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
** IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
** FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
** AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
** LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
** OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
** SOFTWARE.
 */

package oci

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbv1alpha1 "github.com/oracle/oracle-database-operator/apis/database/v1alpha1"
	"github.com/oracle/oracle-database-operator/commons/k8s"
)

type DatabaseService interface {
	CreateAutonomousDatabase(adb *dbv1alpha1.AutonomousDatabase) (database.CreateAutonomousDatabaseResponse, error)
	GetAutonomousDatabase(adbOCID string) (database.GetAutonomousDatabaseResponse, error)
	UpdateAutonomousDatabaseGeneralFields(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateAutonomousDatabaseDBWorkload(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateAutonomousDatabaseLicenseModel(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateAutonomousDatabaseAdminPassword(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateAutonomousDatabaseScalingFields(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateNetworkAccessMTLSRequired(adbOCID string) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateNetworkAccessMTLS(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateNetworkAccessPublic(lastAccessType dbv1alpha1.NetworkAccessTypeEnum, adbOCID string) (resp database.UpdateAutonomousDatabaseResponse, err error)
	UpdateNetworkAccess(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error)
	StartAutonomousDatabase(adbOCID string) (database.StartAutonomousDatabaseResponse, error)
	StopAutonomousDatabase(adbOCID string) (database.StopAutonomousDatabaseResponse, error)
	DeleteAutonomousDatabase(adbOCID string) (database.DeleteAutonomousDatabaseResponse, error)
	DownloadWallet(adb *dbv1alpha1.AutonomousDatabase) (database.GenerateAutonomousDatabaseWalletResponse, error)
	RestoreAutonomousDatabase(adbOCID string, sdkTime common.SDKTime) (database.RestoreAutonomousDatabaseResponse, error)
	ListAutonomousDatabaseBackups(adbOCID string) (database.ListAutonomousDatabaseBackupsResponse, error)
	CreateAutonomousDatabaseBackup(adbBackup *dbv1alpha1.AutonomousDatabaseBackup, adbOCID string) (database.CreateAutonomousDatabaseBackupResponse, error)
	GetAutonomousDatabaseBackup(backupOCID string) (database.GetAutonomousDatabaseBackupResponse, error)
	CreateAutonomousContainerDatabase(acd *dbv1alpha1.AutonomousContainerDatabase) (database.CreateAutonomousContainerDatabaseResponse, error)
	GetAutonomousContainerDatabase(acdOCID string) (database.GetAutonomousContainerDatabaseResponse, error)
	UpdateAutonomousContainerDatabase(acdOCID string, difACD *dbv1alpha1.AutonomousContainerDatabase) (database.UpdateAutonomousContainerDatabaseResponse, error)
	RestartAutonomousContainerDatabase(acdOCID string) (database.RestartAutonomousContainerDatabaseResponse, error)
	TerminateAutonomousContainerDatabase(acdOCID string) (database.TerminateAutonomousContainerDatabaseResponse, error)
}

type databaseService struct {
	logger       logr.Logger
	kubeClient   client.Client
	dbClient     database.DatabaseClient
	vaultService VaultService
}

func NewDatabaseService(
	logger logr.Logger,
	kubeClient client.Client,
	provider common.ConfigurationProvider) (DatabaseService, error) {

	dbClient, err := database.NewDatabaseClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, err
	}

	vaultService, err := NewVaultService(logger, provider)
	if err != nil {
		return nil, err
	}

	return &databaseService{
		logger:       logger.WithName("dbService"),
		kubeClient:   kubeClient,
		dbClient:     dbClient,
		vaultService: vaultService,
	}, nil
}

/********************************
 * Autonomous Database
 *******************************/

// ReadPassword reads the password from passwordSpec, and returns the pointer to the read password string.
// The function returns a nil if nothing is read
func (d *databaseService) readPassword(namespace string, passwordSpec dbv1alpha1.PasswordSpec) (*string, error) {
	logger := d.logger.WithName("readPassword")

	if passwordSpec.K8sSecret.Name != nil {
		logger.Info(fmt.Sprintf("Getting password from Secret %s", *passwordSpec.K8sSecret.Name))

		key := *passwordSpec.K8sSecret.Name
		password, err := k8s.GetSecretValue(d.kubeClient, namespace, *passwordSpec.K8sSecret.Name, key)
		if err != nil {
			return nil, err
		}

		return common.String(password), nil
	}

	if passwordSpec.OCISecret.OCID != nil {
		logger.Info(fmt.Sprintf("Getting password from OCI Vault Secret OCID %s", *passwordSpec.OCISecret.OCID))

		password, err := d.vaultService.GetSecretValue(*passwordSpec.OCISecret.OCID)
		if err != nil {
			return nil, err
		}
		return common.String(password), nil
	}

	return nil, nil
}

func (d *databaseService) readACD_OCID(acd *dbv1alpha1.ACDSpec, namespace string) (*string, error) {
	if acd.OCIACD.OCID != nil {
		return acd.OCIACD.OCID, nil
	}

	if acd.K8sACD.Name != nil {
		fetchedACD := &dbv1alpha1.AutonomousContainerDatabase{}
		if err := k8s.FetchResource(d.kubeClient, namespace, *acd.K8sACD.Name, fetchedACD); err != nil {
			return nil, err
		}

		return fetchedACD.Spec.AutonomousContainerDatabaseOCID, nil
	}

	return nil, nil
}

// CreateAutonomousDatabase sends a request to OCI to provision a database and returns the AutonomousDatabase OCID.
func (d *databaseService) CreateAutonomousDatabase(adb *dbv1alpha1.AutonomousDatabase) (resp database.CreateAutonomousDatabaseResponse, err error) {
	adminPassword, err := d.readPassword(adb.Namespace, adb.Spec.Details.AdminPassword)
	if err != nil {
		return resp, err
	}

	acdOCID, err := d.readACD_OCID(&adb.Spec.Details.AutonomousContainerDatabase, adb.Namespace)
	if err != nil {
		return resp, err
	}

	createAutonomousDatabaseDetails := database.CreateAutonomousDatabaseDetails{
		CompartmentId:                 adb.Spec.Details.CompartmentOCID,
		DbName:                        adb.Spec.Details.DbName,
		CpuCoreCount:                  adb.Spec.Details.CPUCoreCount,
		DataStorageSizeInTBs:          adb.Spec.Details.DataStorageSizeInTBs,
		AdminPassword:                 adminPassword,
		DisplayName:                   adb.Spec.Details.DisplayName,
		IsAutoScalingEnabled:          adb.Spec.Details.IsAutoScalingEnabled,
		IsDedicated:                   adb.Spec.Details.IsDedicated,
		AutonomousContainerDatabaseId: acdOCID,
		DbVersion:                     adb.Spec.Details.DbVersion,
		DbWorkload: database.CreateAutonomousDatabaseBaseDbWorkloadEnum(
			adb.Spec.Details.DbWorkload),
		LicenseModel:             database.CreateAutonomousDatabaseBaseLicenseModelEnum(adb.Spec.Details.LicenseModel),
		IsAccessControlEnabled:   adb.Spec.Details.NetworkAccess.IsAccessControlEnabled,
		WhitelistedIps:           adb.Spec.Details.NetworkAccess.AccessControlList,
		IsMtlsConnectionRequired: adb.Spec.Details.NetworkAccess.IsMTLSConnectionRequired,
		SubnetId:                 adb.Spec.Details.NetworkAccess.PrivateEndpoint.SubnetOCID,
		NsgIds:                   adb.Spec.Details.NetworkAccess.PrivateEndpoint.NsgOCIDs,
		PrivateEndpointLabel:     adb.Spec.Details.NetworkAccess.PrivateEndpoint.HostnamePrefix,

		FreeformTags: adb.Spec.Details.FreeformTags,
	}

	createAutonomousDatabaseRequest := database.CreateAutonomousDatabaseRequest{
		CreateAutonomousDatabaseDetails: createAutonomousDatabaseDetails,
	}

	resp, err = d.dbClient.CreateAutonomousDatabase(context.TODO(), createAutonomousDatabaseRequest)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

func (d *databaseService) GetAutonomousDatabase(adbOCID string) (database.GetAutonomousDatabaseResponse, error) {
	getAutonomousDatabaseRequest := database.GetAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
	}

	return d.dbClient.GetAutonomousDatabase(context.TODO(), getAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateAutonomousDatabaseGeneralFields(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			DisplayName:  difADB.Spec.Details.DisplayName,
			DbName:       difADB.Spec.Details.DbName,
			DbVersion:    difADB.Spec.Details.DbVersion,
			FreeformTags: difADB.Spec.Details.FreeformTags,
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateAutonomousDatabaseDBWorkload(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			DbWorkload: database.UpdateAutonomousDatabaseDetailsDbWorkloadEnum(difADB.Spec.Details.DbWorkload),
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateAutonomousDatabaseLicenseModel(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			LicenseModel: database.UpdateAutonomousDatabaseDetailsLicenseModelEnum(difADB.Spec.Details.LicenseModel),
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateAutonomousDatabaseAdminPassword(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	adminPassword, err := d.readPassword(difADB.Namespace, difADB.Spec.Details.AdminPassword)
	if err != nil {
		return resp, err
	}

	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			AdminPassword: adminPassword,
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateAutonomousDatabaseScalingFields(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			DataStorageSizeInTBs: difADB.Spec.Details.DataStorageSizeInTBs,
			CpuCoreCount:         difADB.Spec.Details.CPUCoreCount,
			IsAutoScalingEnabled: difADB.Spec.Details.IsAutoScalingEnabled,
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateNetworkAccessMTLSRequired(adbOCID string) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			IsMtlsConnectionRequired: common.Bool(true),
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateNetworkAccessMTLS(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			IsMtlsConnectionRequired: difADB.Spec.Details.NetworkAccess.IsMTLSConnectionRequired,
		},
	}
	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateNetworkAccessPublic(
	lastAccessType dbv1alpha1.NetworkAccessTypeEnum,
	adbOCID string) (resp database.UpdateAutonomousDatabaseResponse, err error) {

	updateAutonomousDatabaseDetails := database.UpdateAutonomousDatabaseDetails{}

	if lastAccessType == dbv1alpha1.NetworkAccessTypeRestricted {
		updateAutonomousDatabaseDetails.WhitelistedIps = []string{""}
	} else if lastAccessType == dbv1alpha1.NetworkAccessTypePrivate {
		updateAutonomousDatabaseDetails.PrivateEndpointLabel = common.String("")
	}

	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId:            common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: updateAutonomousDatabaseDetails,
	}

	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) UpdateNetworkAccess(adbOCID string, difADB *dbv1alpha1.AutonomousDatabase) (resp database.UpdateAutonomousDatabaseResponse, err error) {
	updateAutonomousDatabaseRequest := database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		UpdateAutonomousDatabaseDetails: database.UpdateAutonomousDatabaseDetails{
			IsAccessControlEnabled: difADB.Spec.Details.NetworkAccess.IsAccessControlEnabled,
			WhitelistedIps:         difADB.Spec.Details.NetworkAccess.AccessControlList,
			SubnetId:               difADB.Spec.Details.NetworkAccess.PrivateEndpoint.SubnetOCID,
			NsgIds:                 difADB.Spec.Details.NetworkAccess.PrivateEndpoint.NsgOCIDs,
			PrivateEndpointLabel:   difADB.Spec.Details.NetworkAccess.PrivateEndpoint.HostnamePrefix,
		},
	}

	return d.dbClient.UpdateAutonomousDatabase(context.TODO(), updateAutonomousDatabaseRequest)
}

func (d *databaseService) StartAutonomousDatabase(adbOCID string) (database.StartAutonomousDatabaseResponse, error) {
	startRequest := database.StartAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
	}

	return d.dbClient.StartAutonomousDatabase(context.TODO(), startRequest)
}

func (d *databaseService) StopAutonomousDatabase(adbOCID string) (database.StopAutonomousDatabaseResponse, error) {
	stopRequest := database.StopAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
	}

	return d.dbClient.StopAutonomousDatabase(context.TODO(), stopRequest)
}

func (d *databaseService) DeleteAutonomousDatabase(adbOCID string) (database.DeleteAutonomousDatabaseResponse, error) {
	deleteRequest := database.DeleteAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
	}

	return d.dbClient.DeleteAutonomousDatabase(context.TODO(), deleteRequest)
}

func (d *databaseService) DownloadWallet(adb *dbv1alpha1.AutonomousDatabase) (resp database.GenerateAutonomousDatabaseWalletResponse, err error) {
	// Prepare wallet password
	walletPassword, err := d.readPassword(adb.Namespace, adb.Spec.Details.Wallet.Password)
	if err != nil {
		return resp, err
	}

	// Download a Wallet
	req := database.GenerateAutonomousDatabaseWalletRequest{
		AutonomousDatabaseId: adb.Spec.Details.AutonomousDatabaseOCID,
		GenerateAutonomousDatabaseWalletDetails: database.GenerateAutonomousDatabaseWalletDetails{
			Password: walletPassword,
		},
	}

	// Send the request using the service client
	resp, err = d.dbClient.GenerateAutonomousDatabaseWallet(context.TODO(), req)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

/********************************
 * Autonomous Database Restore
 *******************************/

func (d *databaseService) RestoreAutonomousDatabase(adbOCID string, sdkTime common.SDKTime) (database.RestoreAutonomousDatabaseResponse, error) {
	request := database.RestoreAutonomousDatabaseRequest{
		AutonomousDatabaseId: common.String(adbOCID),
		RestoreAutonomousDatabaseDetails: database.RestoreAutonomousDatabaseDetails{
			Timestamp: &sdkTime,
		},
	}
	return d.dbClient.RestoreAutonomousDatabase(context.TODO(), request)
}

/********************************
 * Autonomous Database Backup
 *******************************/

func (d *databaseService) ListAutonomousDatabaseBackups(adbOCID string) (database.ListAutonomousDatabaseBackupsResponse, error) {
	listBackupRequest := database.ListAutonomousDatabaseBackupsRequest{
		AutonomousDatabaseId: common.String(adbOCID),
	}

	return d.dbClient.ListAutonomousDatabaseBackups(context.TODO(), listBackupRequest)
}

func (d *databaseService) CreateAutonomousDatabaseBackup(adbBackup *dbv1alpha1.AutonomousDatabaseBackup, adbOCID string) (database.CreateAutonomousDatabaseBackupResponse, error) {
	createBackupRequest := database.CreateAutonomousDatabaseBackupRequest{
		CreateAutonomousDatabaseBackupDetails: database.CreateAutonomousDatabaseBackupDetails{
			AutonomousDatabaseId:  common.String(adbOCID),
			IsLongTermBackup:      adbBackup.Spec.IsLongTermBackup,
			RetentionPeriodInDays: adbBackup.Spec.RetentionPeriodInDays,
		},
	}

	// Use the spec.displayName as the displayName of the backup if is provided,
	// otherwise use the resource name as the displayName.
	if adbBackup.Spec.DisplayName != nil {
		createBackupRequest.DisplayName = adbBackup.Spec.DisplayName
	} else {
		createBackupRequest.DisplayName = common.String(adbBackup.GetName())
	}

	return d.dbClient.CreateAutonomousDatabaseBackup(context.TODO(), createBackupRequest)
}

func (d *databaseService) GetAutonomousDatabaseBackup(backupOCID string) (database.GetAutonomousDatabaseBackupResponse, error) {
	getBackupRequest := database.GetAutonomousDatabaseBackupRequest{
		AutonomousDatabaseBackupId: common.String(backupOCID),
	}

	return d.dbClient.GetAutonomousDatabaseBackup(context.TODO(), getBackupRequest)
}
