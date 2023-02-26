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
	HostURL    string
	HTTPClient *http.Client
	AccessKey  string
}

// NewClient -
func NewClient(customDomain string, accessKey string) (*Client, error) {
	c := Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		AccessKey: accessKey,
		HostURL: customDomain,
	}

	return &c, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer " + c.AccessKey)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}
