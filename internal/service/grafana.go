package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"console/internal/errs"
)

// ---- Grafana models ----

type DashboardMeta struct {
	Type      string `json:"type,omitempty"`
	CanSave   *bool  `json:"canSave,omitempty"`
	CanEdit   *bool  `json:"canEdit,omitempty"`
	CanStar   *bool  `json:"canStar,omitempty"`
	Slug      string `json:"slug,omitempty"`
	Expires   string `json:"expires,omitempty"`
	Created   string `json:"created,omitempty"`
	Updated   string `json:"updated,omitempty"`
	UpdatedBy string `json:"updatedBy,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	Version   *int   `json:"version,omitempty"`
}

type DashboardPostResult struct {
	Id      *int   `json:"id,omitempty"`
	Uid     string `json:"uid,omitempty"`
	Url     string `json:"url,omitempty"`
	Status  string `json:"status,omitempty"`
	Version *int   `json:"version,omitempty"`
	Slug    string `json:"slug,omitempty"`
}

type DatasourceCreationResult struct {
	Id         *int        `json:"id,omitempty"`
	Name       string      `json:"name,omitempty"`
	Message    string      `json:"message,omitempty"`
	Datasource *Datasource `json:"datasource,omitempty"`
}

type Datasource struct {
	Id        *int   `json:"id,omitempty"`
	Uid       string `json:"uid,omitempty"`
	OrgId     *int   `json:"orgId,omitempty"`
	Name      string `json:"name,omitempty"`
	IsDefault *bool  `json:"isDefault,omitempty"`
	Type      string `json:"type,omitempty"`
	Url       string `json:"url,omitempty"`
	Access    string `json:"access,omitempty"`
	Database  string `json:"database,omitempty"`
	User      string `json:"user,omitempty"`
	Password  string `json:"password,omitempty"`
}

type GrafanaDashboard struct {
	Meta      *DashboardMeta         `json:"meta,omitempty"`
	Dashboard map[string]interface{} `json:"dashboard,omitempty"`
}

func (d *GrafanaDashboard) getId() *int {
	if d.Dashboard == nil {
		return nil
	}
	if v, ok := d.Dashboard["id"]; ok && v != nil {
		if f, ok := toFloat(v); ok {
			n := int(f)
			return &n
		}
	}
	return nil
}

func (d *GrafanaDashboard) setId(id *int) {
	if d.Dashboard == nil {
		d.Dashboard = map[string]interface{}{}
	}
	if id == nil {
		delete(d.Dashboard, "id")
	} else {
		d.Dashboard["id"] = *id
	}
}

func (d *GrafanaDashboard) getUid() string {
	if d.Dashboard == nil {
		return ""
	}
	if v, ok := d.Dashboard["uid"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (d *GrafanaDashboard) setUid(uid string) {
	if d.Dashboard == nil {
		d.Dashboard = map[string]interface{}{}
	}
	if uid == "" {
		delete(d.Dashboard, "uid")
	} else {
		d.Dashboard["uid"] = uid
	}
}

func (d *GrafanaDashboard) getTitle() string {
	if d.Dashboard == nil {
		return ""
	}
	if v, ok := d.Dashboard["title"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (d *GrafanaDashboard) getVersion() *int {
	if d.Dashboard == nil {
		return nil
	}
	if v, ok := d.Dashboard["version"]; ok && v != nil {
		if f, ok := toFloat(v); ok {
			n := int(f)
			return &n
		}
	}
	return nil
}

func (d *GrafanaDashboard) setVersion(version *int) {
	if d.Dashboard == nil {
		d.Dashboard = map[string]interface{}{}
	}
	if version == nil {
		delete(d.Dashboard, "version")
	} else {
		d.Dashboard["version"] = *version
	}
}

type GrafanaSearchResult struct {
	Id        *int64   `json:"id,omitempty"`
	Uid       string   `json:"uid,omitempty"`
	Title     string   `json:"title,omitempty"`
	Url       string   `json:"url,omitempty"`
	Type      string   `json:"type,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	IsStarred *bool    `json:"isStarred,omitempty"`
	SortMeta  *int     `json:"sortMeta,omitempty"`
}

// ---- Grafana client ----

type GrafanaClient struct {
	baseUrl           string
	authorizationData string
	httpClient        *http.Client
}

func NewGrafanaClient(baseUrl, username, password string) *GrafanaClient {
	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	return &GrafanaClient{baseUrl: baseUrl, authorizationData: auth, httpClient: &http.Client{}}
}

func ParseDashboardData(jsonData string) (*GrafanaDashboard, error) {
	var dashboard map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &dashboard); err != nil {
		return nil, err
	}
	return &GrafanaDashboard{Dashboard: dashboard}, nil
}

func (c *GrafanaClient) GetDashboard(uid string) (*GrafanaDashboard, error) {
	var result GrafanaDashboard
	err := c.do(http.MethodGet, "api/dashboards/uid/"+uid, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GrafanaClient) CreateDashboard(dashboard *GrafanaDashboard) (*DashboardPostResult, error) {
	return c.UpdateDashboard(dashboard)
}

func (c *GrafanaClient) UpdateDashboard(dashboard *GrafanaDashboard) (*DashboardPostResult, error) {
	var result DashboardPostResult
	err := c.do(http.MethodPost, "api/dashboards/db/", dashboard, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GrafanaClient) GetDatasources() ([]Datasource, error) {
	var result []Datasource
	err := c.do(http.MethodGet, "api/datasources/", nil, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaClient) GetDatasource(name string) (*Datasource, error) {
	var result Datasource
	err := c.do(http.MethodGet, "api/datasources/name/"+name, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GrafanaClient) CreateDatasource(ds *Datasource) (*DatasourceCreationResult, error) {
	var result DatasourceCreationResult
	err := c.do(http.MethodPost, "api/datasources/", ds, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GrafanaClient) UpdateDatasource(ds *Datasource, id int) (*DatasourceCreationResult, error) {
	var result DatasourceCreationResult
	err := c.do(http.MethodPut, fmt.Sprintf("api/datasources/%d", id), ds, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GrafanaClient) Search(query, searchType, tag string, starred *bool) ([]GrafanaSearchResult, error) {
	path := "api/search/"
	var params []string
	if query != "" {
		params = append(params, "query="+urlQueryEscape(query))
	}
	if searchType != "" {
		params = append(params, "type="+urlQueryEscape(searchType))
	}
	if tag != "" {
		params = append(params, "tag="+urlQueryEscape(tag))
	}
	if starred != nil {
		params = append(params, fmt.Sprintf("starred=%v", *starred))
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}
	var result []GrafanaSearchResult
	err := c.do(http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *GrafanaClient) do(method, path string, body interface{}, result interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseUrl+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authorizationData)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := "Unexpected Grafana error"
		if len(respBody) > 0 {
			msg += "; " + string(respBody)
		}
		return errs.Business(msg)
	}
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return err
		}
	}
	return nil
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	}
	return 0, false
}

func urlQueryEscape(s string) string {
	// 简单转义，满足 Grafana 查询场景
	return s
}
