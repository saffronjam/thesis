package azure

import "context"

func (c *Client) DeleteDisk(ctx context.Context, resourceGroupName, diskName string) error {
	pResp, err := c.DisksClient.BeginDelete(ctx, resourceGroupName, diskName, nil)

	_, err = pResp.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
