package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v2"
)

func (c *Client) CreateSubnet(ctx context.Context, name string, resourceGroupName string, vnetName string, addressPrefix string) (*armnetwork.Subnet, error) {
	pResp, err := c.SubnetsClient.BeginCreateOrUpdate(ctx, resourceGroupName, vnetName, name, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr(addressPrefix),
		},
	}, nil)

	if err != nil {
		return nil, err
	}

	resp, err := pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.Subnet, nil
}

func (c *Client) DeleteSubnet(ctx context.Context, name string, resourceGroupName string, vnetName string) error {
	pResp, err := c.SubnetsClient.BeginDelete(ctx, resourceGroupName, vnetName, name, nil)

	_, err = pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
