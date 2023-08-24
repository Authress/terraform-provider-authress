package authress

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// HostURL - Default Authress URL
const HostURL string = "http://localhost:19090"

// Client -
type Client struct {
	HostURL    	string
	HTTPClient 	*http.Client
	AccessKey  	string
	Version		string
}

// NewClient -
func NewClient(customDomain string, accessKey string, version string) (*Client, error) {
	c := Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		AccessKey: accessKey,
		HostURL: customDomain,
		Version: version,
	}

	return &c, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, int, error) {
	req.Header.Set("Authorization", "Bearer " + c.AccessKey)
	req.Header.Set("User-Agent", "Authress SDK; Terraform; " + c.Version + ";")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, res.StatusCode, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, res.StatusCode, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, res.StatusCode, err
}
