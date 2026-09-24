package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/linode/linodego/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNATGateway(
	t *testing.T,
	fixtureYaml string,
	modifiers ...func(*linodego.NATGatewayCreateOptions),
) (*linodego.Client, *linodego.NATGateway) {
	t.Helper()
	client, fixtureTeardown := createTestClient(t, fixtureYaml)
	createOpts := linodego.NATGatewayCreateOptions{
		Label:  "go-test-nat-gateway-" + randLabel(),
		Region: getRegionsWithCaps(t, client, []linodego.RegionCapability{linodego.CapabilityNATGateway})[0],
	}

	for _, modifier := range modifiers {
		modifier(&createOpts)
	}

	gateway, err := client.CreateNATGateway(context.Background(), createOpts)
	require.NoErrorf(t, err, "Error creating NAT Gateway: %v", err)

	t.Cleanup(func() {
		if err := client.DeleteNATGateway(context.Background(), gateway.ID); err != nil {
			t.Errorf("Error deleting NAT Gateway: %v", err)
		}
		fixtureTeardown()
	})
	return client, gateway
}

func setupNATGatewayReservedIP(t *testing.T, client *linodego.Client, region string) *linodego.InstanceIP {
	t.Helper()
	reservedIP, err := client.ReserveIPAddress(context.Background(), linodego.ReserveIPOptions{
		Region: region,
	})
	require.NoErrorf(t, err, "Error creating NAT Gateway: %v", err)

	t.Cleanup(func() {
		if err := client.DeleteReservedIPAddress(context.Background(), reservedIP.Address); err != nil {
			t.Errorf("Failed to delete reserved IP %s: %v", reservedIP.Address, err)
		}
	})
	return reservedIP
}

func verifyNATGateway(t *testing.T, gatewayCreated *linodego.NATGateway, gatewayRead *linodego.NATGateway) {
	assert.Equal(t, gatewayCreated.ID, gatewayRead.ID, "NAT Gateway ID mismatch")
	assert.Equal(t, gatewayCreated.Region, gatewayRead.Region, "NAT Gateway region mismatch")
	assert.Equal(t, gatewayCreated.Label, gatewayRead.Label, "NAT Gateway label mismatch")
	assert.Equal(t, 0, len(gatewayRead.Addresses), "Expected 0 addresses in NAT Gateway")
	assert.Greater(t, gatewayRead.AddressAutoscaleMax, 0, "Expected address autoscale max to be greater than 0 in NAT Gateway")
	assert.Equal(t, 4096, gatewayRead.DefaultPortsPerInterface, "Expected default ports per interface of 4096 in NAT Gateway")
	assert.Equal(t, 0, gatewayRead.PortsetAssignments, "Expected 0 portset assignment in NAT Gateway")
	assert.Greater(t, gatewayRead.PortsetCapacity, 0, "Expected portset capacity to be greater than 0 in NAT Gateway")
}

func verifyNATGatewayAddress(t *testing.T, addressNAT *linodego.NATGatewayAddressObject, addressReserved string) {
	assert.Equal(t, addressReserved, addressNAT.Address, "NAT Gateway address mismatch")
	assert.Equal(t, false, addressNAT.InUse, "Expected NAT Gateway address to not be in use")
	assert.Equal(t, 0, addressNAT.InterfaceCount, "Expected 0 interfaces for NAT Gateway address")
	assert.True(t, strings.HasSuffix(addressNAT.InterfaceURL, fmt.Sprintf("/%s/interfaces", addressReserved)), "Expected NAT Gateway interface URL to end with /<address>/interfaces")
	assert.Equal(t, 0, addressNAT.PortsetAssignments, "Expected 0 portset assignments for NAT Gateway address")
	assert.Greater(t, addressNAT.PortsetCapacity, 0, "Expected NAT Gateway address portset capacity to be greater than 0")
}

func TestNATGateway_Create_smoke(t *testing.T) {
	client, gatewayCreated := setupNATGateway(t, "fixtures/TestNATGateway_Create_smoke.yaml")

	gateway, err := client.GetNATGateway(context.Background(), gatewayCreated.ID)
	require.NoErrorf(t, err, "Error retrieving NAT Gateway: %v", err)
	verifyNATGateway(t, gatewayCreated, gateway)

	f := linodego.Filter{}
	f.AddField(linodego.Eq, "label", gatewayCreated.Label)
	filter, err := f.MarshalJSON()
	require.NoErrorf(t, err, "Error marshalling filter: %v", err)

	gateways, err := client.ListNATGateways(context.Background(), &linodego.ListOptions{Filter: string(filter)})
	require.NoErrorf(t, err, "Error listing NAT Gateways: %v", err)
	require.Equal(t, 1, len(gateways), "Expected exactly one NAT Gateway in the list")
	verifyNATGateway(t, gatewayCreated, &gateways[0])
}

func TestNATGateway_Update(t *testing.T) {
	client, gatewayCreated := setupNATGateway(t, "fixtures/TestNATGateway_Update.yaml")

	newLabel := gatewayCreated.Label + "-updated"
	updateOpts := linodego.NATGatewayUpdateOptions{Label: &newLabel}
	gatewayUpdated, err := client.UpdateNATGateway(context.Background(), gatewayCreated.ID, updateOpts)
	require.NoErrorf(t, err, "Error updating NAT Gateway: %v", err)

	gateway, err := client.GetNATGateway(context.Background(), gatewayUpdated.ID)
	require.NoErrorf(t, err, "Error retrieving NAT Gateway: %v", err)
	assert.Equal(t, newLabel, gateway.Label, "Expected updated NAT Gateway label")
}

func TestNATGateway_AssignReservedIP(t *testing.T) {
	client, gatewayCreated := setupNATGateway(
		t,
		"fixtures/TestNATGateway_Assign_ReservedIP.yaml",
		func(opts *linodego.NATGatewayCreateOptions) {
			opts.UseAutoscaling = linodego.Pointer(false)
		},
	)
	reservedIP := setupNATGatewayReservedIP(t, client, gatewayCreated.Region)

	_, err := client.NATGatewayAddAddress(
		context.Background(),
		gatewayCreated.ID,
		linodego.NATGatewayAddAddressOptions{
			Address: reservedIP.Address,
		},
	)
	require.NoErrorf(t, err, "Error adding Reserved IP address to NAT Gateway: %v", err)

	addresses, err := client.NATGatewayListAddresses(context.Background(), gatewayCreated.ID, nil)
	require.NoErrorf(t, err, "Error retrieving list of NAT Gateway addresses: %v", err)
	require.Len(t, addresses, 1, "List of NAT Gateway addresses should contain 1 element only")
	verifyNATGatewayAddress(t, &addresses[0], reservedIP.Address)

	address, err := client.NATGatewayGetAddress(context.Background(), gatewayCreated.ID, reservedIP.Address)
	require.NoErrorf(t, err, "Error retrieving a NAT Gateway address: %v", err)
	verifyNATGatewayAddress(t, address, reservedIP.Address)

	err = client.NATGatewayDeleteAddress(context.Background(), gatewayCreated.ID, reservedIP.Address)
	require.NoErrorf(t, err, "Error deleting address form NAT Gateway: %v", err)

	addresses, err = client.NATGatewayListAddresses(context.Background(), gatewayCreated.ID, nil)
	require.NoErrorf(t, err, "Error retrieving list of NAT Gateway addresses: %v", err)
	require.Len(t, addresses, 0, "List of NAT Gateway addresses should contain 0 element only")
}

func TestNATGateway_GetLinodeInterfaces(t *testing.T) {}

func TestNATGateway_GetTypes(t *testing.T) {}

func TestNATGateway_GetSettings(t *testing.T) {}

// TODO: Move to VPC tests
func TestNATGateway_VPCSubnetWithNATGateway(t *testing.T) {}
