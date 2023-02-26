package authress

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (c *Client) GetRoles() ([]Role, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/roles", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	roles := []Role{}
	err = json.Unmarshal(body, &roles)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *Client) GetRole(roleID string) ([]Role, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/roles/%s", c.HostURL, roleID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	roles := []Role{}
	err = json.Unmarshal(body, &roles)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *Client) CreateRole(role Role) (*Role, error) {
	rb, err := json.Marshal(role)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/roles", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	newRole := Role{}
	err = json.Unmarshal(body, &newRole)
	if err != nil {
		return nil, err
	}

	return &newRole, nil
}

func (c *Client) UpdateRole(roleID string, role Role) (*Role, error) {
	rb, err := json.Marshal(role)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/v1/roles/%s", c.HostURL, roleID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	newRole := Role{}
	err = json.Unmarshal(body, &newRole)
	if err != nil {
		return nil, err
	}

	return &newRole, nil
}


func (c *Client) DeleteRole(roleID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/roles/%s", c.HostURL, roleID), nil)
	if err != nil {
		return err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return err
	}

	return nil
}
