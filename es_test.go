package vulcanizer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

// ServerSetup type contains the Method, Path, Body and Response strings, as well as the HTTP Status code.
type ServerSetup struct {
	Method, Path, Body, Response string
	HTTPStatus                   int
}

// setupTestServer sets up an HTTP test server to serve data to the test that come after it.

func setupTestServers(t *testing.T, setups []*ServerSetup) (string, int, *httptest.Server) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		requestBytes, _ := ioutil.ReadAll(r.Body)
		requestBody := string(requestBytes)

		matched := false
		for _, setup := range setups {
			if r.Method == setup.Method && r.URL.EscapedPath() == setup.Path && requestBody == setup.Body {
				matched = true
				if setup.HTTPStatus == 0 {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(setup.HTTPStatus)
				}
				_, err := w.Write([]byte(setup.Response))
				if err != nil {
					t.Fatalf("Unable to write test server response: %v", err)
				}
			}
		}

		if !matched {
			t.Fatalf("No requests matched setup. Got method %s, Path %s, body %s", r.Method, r.URL.EscapedPath(), requestBody)
		}
	}))
	url, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(url.Port())
	return url.Hostname(), port, ts
}

func TestGetClusterExcludeSettings(t *testing.T) {

	testSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_host":"excluded.host","_name":"excluded_name","_ip":"10.0.0.99"}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()

	client := NewClient(host, port)

	excludeSettings, err := client.GetClusterExcludeSettings()

	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if excludeSettings.Ips[0] != "10.0.0.99" || len(excludeSettings.Ips) != 1 {
		t.Errorf("Expected 10.0.0.99 for excluded ip, got %s", excludeSettings.Ips)
	}

	if excludeSettings.Names[0] != "excluded_name" || len(excludeSettings.Names) != 1 {
		t.Errorf("Expected excluded_name for excluded name, got %s", excludeSettings.Names)
	}

	if excludeSettings.Hosts[0] != "excluded.host" || len(excludeSettings.Hosts) != 1 {
		t.Errorf("Expected excluded.host for excluded host, got %s", excludeSettings.Hosts)
	}
}

func TestDrainServer_OneValue(t *testing.T) {

	getSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":""}}}}}}`,
	}

	putSetup := &ServerSetup{
		Method:   "PUT",
		Path:     "/_cluster/settings",
		Body:     `{"transient":{"cluster.routing.allocation.exclude._name":"server_to_drain"}}`,
		Response: `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"server_to_drain"}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
	defer ts.Close()
	client := NewClient(host, port)

	excludeSettings, err := client.DrainServer("server_to_drain")

	if err != nil {
		t.Errorf("Unexpected error, %s", err)
	}

	if excludeSettings.Names[0] != "server_to_drain" {
		t.Errorf("Expected response, got %+v", excludeSettings)
	}
}

func TestDrainServer_ExistingValues(t *testing.T) {

	getSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"existing_one,existing_two"}}}}}}`,
	}

	putSetup := &ServerSetup{
		Method:   "PUT",
		Path:     "/_cluster/settings",
		Body:     `{"transient":{"cluster.routing.allocation.exclude._name":"existing_one,existing_two,server_to_drain"}}`,
		Response: `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"server_to_drain,existing_one,existing_two"}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
	defer ts.Close()
	client := NewClient(host, port)

	excludeSettings, err := client.DrainServer("server_to_drain")
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if len(excludeSettings.Names) != 3 || excludeSettings.Names[2] != "server_to_drain" {
		t.Errorf("unexpected response, got %+v", excludeSettings)
	}
}

func TestFillOneServer_ExistingServers(t *testing.T) {

	getSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"excluded_server1,good_server,excluded_server2"}}}}}}`,
	}

	putSetup := &ServerSetup{
		Method:   "PUT",
		Path:     "/_cluster/settings",
		Body:     `{"transient":{"cluster.routing.allocation.exclude._name":"excluded_server1,excluded_server2"}}`,
		Response: `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"excluded_server1,excluded_server2"}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
	defer ts.Close()
	client := NewClient(host, port)

	excludeSettings, err := client.FillOneServer("good_server")

	if err != nil {
		t.Errorf("Unexpected error expected nil, got %s", err)
	}

	if excludeSettings.Names[0] != "excluded_server1" {
		t.Errorf("unexpected response, got %+v", excludeSettings)
	}
}

func TestFillOneServer_OneServer(t *testing.T) {

	getSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"good_server"}}}}}}`,
	}

	putSetup := &ServerSetup{
		Method:   "PUT",
		Path:     "/_cluster/settings",
		Body:     `{"transient":{"cluster.routing.allocation.exclude._name":""}}`,
		Response: `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":""}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
	defer ts.Close()
	client := NewClient(host, port)

	_, err := client.FillOneServer("good_server")

	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}
}

func TestFillAll(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "PUT",
		Path:     "/_cluster/settings",
		Body:     `{"transient":{"cluster.routing.allocation.exclude":{"_host":"","_ip":"","_name":""}}}`,
		Response: `{"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"", "_ip": "", "_host": ""}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	excludeSettings, err := client.FillAll()
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if len(excludeSettings.Ips) != 0 {
		t.Errorf("Expected empty excluded Ips, got %s", excludeSettings.Ips)
	}

	if len(excludeSettings.Names) != 0 {
		t.Errorf("Expected empty excluded Names, got %s", excludeSettings.Names)
	}

	if len(excludeSettings.Hosts) != 0 {
		t.Errorf("Expected empty excluded Hosts, got %s", excludeSettings.Hosts)
	}
}

func TestGetNodes(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cat/nodes",
		Response: `[{"master": "*", "role": "d", "name": "foo", "ip": "127.0.0.1", "id": "abc"}]`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	nodes, err := client.GetNodes()

	if err != nil {
		t.Errorf("Unexpected error expected nil, got %s", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Unexpected nodes, got %s", nodes)
	}

	if nodes[0].Name != "foo" {
		t.Errorf("Unexpected node name, expected foo, got %s", nodes[0].Name)
	}
}

func TestGetIndices(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cat/indices",
		Response: `[{"health":"yellow","status":"open","index":"index1","pri":"5","rep":"1","store.size":"3.6kb", "docs.count":"1500"}]`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	indices, err := client.GetIndices()

	if err != nil {
		t.Errorf("Unexpected error expected nil, got %s", err)
	}

	if len(indices) != 1 {
		t.Errorf("Unexpected indices, got %v", indices)
	}

	if indices[0].Health != "yellow" || indices[0].ReplicaCount != 1 || indices[0].DocumentCount != 1500 {
		t.Errorf("Unexpected index values, got %v", indices[0])
	}
}

func TestGetHealth(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cat/health",
		Response: `[{"cluster":"elasticsearch_nickcanz","status":"yellow","relo":"0","init":"0","unassign":"5","pending_tasks":"0","active_shards_percent":"50.0%"}]`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	health, err := client.GetHealth()

	if err != nil {
		t.Errorf("Unexpected error expected nil, got %s", err)
	}

	if len(health) != 1 {
		t.Errorf("Unexpected health, got %+v", health)
	}

	if health[0].UnassignedShards != 5 {
		t.Errorf("Unexpected unassigned shards, expected 5, got %d", health[0].UnassignedShards)
	}
}

func TestGetSettings(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_host":"","_name":"10.0.0.2","_ip":""}}}}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	clusterSettings, err := client.GetSettings()

	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if len(clusterSettings.PersistentSettings) != 0 {
		t.Errorf("Unexpected persistent settings, got %v", clusterSettings.PersistentSettings)
	}

	if len(clusterSettings.TransientSettings) != 1 {
		t.Errorf("Unexpected transient settings, got %v", clusterSettings.TransientSettings)
	}

	if clusterSettings.TransientSettings[0].Setting != "cluster.routing.allocation.exclude._name" {
		t.Errorf("Unexpected setting name, got %s", clusterSettings.TransientSettings[0].Setting)
	}

	if clusterSettings.TransientSettings[0].Value != "10.0.0.2" {
		t.Errorf("Unexpected setting value, got %s", clusterSettings.TransientSettings[0].Value)
	}
}

// TestSetSetting Func is an integration test for all things that use the SetSetting functionality.
func TestSetSettings(t *testing.T) {

	tt := []struct {
		Name        string
		Method      string
		Body        string
		GetResponse string
		PutResponse string
		Setting     string
		SetValue    string
		HTTPStatus  int
		OldValue    string
	}{
		{
			// Tests for behavior with existing transient setting.
			Name:        "Existing Transient Setting",
			GetResponse: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"10.0.0.2"}}}}}}`,
			Body:        `{"transient":{"cluster.routing.allocation.exclude._name":"10.0.0.99"}}`,
			PutResponse: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"10.0.0.99"}}}}}}`,
			Setting:     "cluster.routing.allocation.exclude._name",
			SetValue:    "10.0.0.99",
			OldValue:    "10.0.0.2",
		},

		{
			// Tests for behavior with existing persistent settings.
			Name:        "Existing Persistent Setting",
			GetResponse: `{"transient":{},"persistent":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"10.0.0.2"}}}}}}`,
			Body:        `{"transient":{"cluster.routing.allocation.exclude._name":"10.0.0.99"}}`,
			PutResponse: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"10.0.0.99"}}}}}}`,
			Setting:     "cluster.routing.allocation.exclude._name",
			SetValue:    "10.0.0.99",
			OldValue:    "10.0.0.2",
		},

		{
			// Tests for behavior with NO existing persistent settings.
			Name:        "No existing settings",
			GetResponse: `{"transient":{},"persistent":{}}`,
			Body:        `{"transient":{"cluster.routing.allocation.exclude._name":"10.0.0.99"}}`,
			PutResponse: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"exclude":{"_name":"10.0.0.99"}}}}}}`,
			Setting:     "cluster.routing.allocation.exclude._name",
			SetValue:    "10.0.0.99",
			OldValue:    "",
		},
	}

	for _, x := range tt {
		t.Run(x.Name, func(st *testing.T) {

			getSetup := &ServerSetup{
				Method:   "GET",
				Path:     "/_cluster/settings",
				Response: x.GetResponse,
			}
			putSetup := &ServerSetup{
				Method:   "PUT",
				Path:     "/_cluster/settings",
				Body:     x.Body,
				Response: x.PutResponse,
			}

			host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
			defer ts.Close()
			client := NewClient(host, port)

			oldSetting, newSetting, err := client.SetSetting(x.Setting, x.SetValue)

			if err != nil {
				st.Errorf("Expected error to be nil, %s", err)
			}

			if oldSetting != x.OldValue {
				st.Errorf("Unexpected old value, got %s", oldSetting)
			}

			if newSetting != "10.0.0.99" {
				st.Errorf("Unexpected new value, got %s", newSetting)
			}

		})
	}
}

// TestSetSetting Func is an integration test for all things that use the SetAllocation functionality.
func TestAllocationSettings(t *testing.T) {

	tt := []struct {
		Name     string
		Path     string
		Body     string
		Response string
		Setting  string
		Expected string
	}{
		{
			// Allocation Enable.
			Name:     "Allocation Enable",
			Body:     `{"transient":{"cluster.routing.allocation.enable":"all"}}`,
			Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"enable": "all"}}}}}`,
			Setting:  "enable",
			Expected: "all",
		},

		{
			// Allocation Disable.
			Name:     "Allocation Disable",
			Body:     `{"transient":{"cluster.routing.allocation.enable":"none"}}`,
			Response: `{"persistent":{},"transient":{"cluster":{"routing":{"allocation":{"enable": "none"}}}}}`,
			Setting:  "disable",
			Expected: "none",
		},
	}

	for _, x := range tt {
		t.Run(x.Name, func(st *testing.T) {

			testSetup := &ServerSetup{
				Method:   "PUT",
				Path:     "/_cluster/settings",
				Body:     x.Body,
				Response: x.Response,
			}

			host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
			defer ts.Close()
			client := NewClient(host, port)

			resp, err := client.SetAllocation(x.Setting)

			if err != nil {
				st.Errorf("Unexpected error, got %s", err)
			}

			if resp != x.Expected {
				st.Errorf("Unexpected response, got %s", resp)
			}

		})
	}
}

func TestSetSetting_BadRequest(t *testing.T) {
	getSetup := &ServerSetup{
		Method:   "GET",
		Path:     "/_cluster/settings",
		Response: `{"transient":{},"persistent":{}}`,
	}
	putSetup := &ServerSetup{
		Method:     "PUT",
		Path:       "/_cluster/settings",
		Body:       `{"transient":{"cluster.routing.allocation.enable":"foo"}}`,
		HTTPStatus: http.StatusBadRequest,
		Response:   `{"error":{"root_cause":[{"type":"illegal_argument_exception","reason":"Illegal allocation.enable value [FOO]"}],"type":"illegal_argument_exception","reason":"Illegal allocation.enable value [FOO]"},"status":400}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{getSetup, putSetup})
	defer ts.Close()
	client := NewClient(host, port)

	_, _, err := client.SetSetting("cluster.routing.allocation.enable", "foo")

	if err == nil {
		t.Errorf("Expected error to not be nil, %s", err)
	}

	if err.Error() != fmt.Sprintf("Bad HTTP Status from Elasticsearch: 400, %s", putSetup.Response) {
		t.Errorf("Unexpected error message, %s", err)
	}
}

func TestGetSnapshots(t *testing.T) {
	testSetup := &ServerSetup{
		Method: "GET",
		Path:   "/_snapshot/octocat/_all",
		Response: `{
  "snapshots": [
    {
      "snapshot": "snapshot1",
      "uuid": "kXx-r58tSOeVvDbvCC1IsQ",
      "version_id": 5060699,
      "version": "5.6.6",
      "indices": [ "index1", "index2" ],
      "state": "SUCCESS",
      "start_time": "2018-04-03T06:06:24.837Z",
      "start_time_in_millis": 1522735584837,
      "end_time": "2018-04-03T07:41:01.719Z",
      "end_time_in_millis": 1522741261719,
      "duration_in_millis": 1000,
      "failures": [],
      "shards": { "total": 93, "failed": 0, "successful": 93 }
    },
    {
      "snapshot": "snapshot2",
      "uuid": "ReLFDkUfQcysi6HG2y40uw",
      "version_id": 5060699,
      "version": "5.6.6",
      "indices": [ "index1", "index2" ],
      "state": "SUCCESS",
      "start_time": "2018-04-03T18:13:11.012Z",
      "start_time_in_millis": 1522779191012,
      "end_time": "2018-04-03T18:25:58.440Z",
      "end_time_in_millis": 1522779958440,
      "duration_in_millis": 500,
      "failures": [],
      "shards": { "total": 93, "failed": 0, "successful": 93 }
    }
  ]
}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	snapshots, err := client.GetSnapshots("octocat")

	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if len(snapshots) != 2 {
		t.Errorf("Unexpected snapshots, got %v", snapshots)
	}

	if snapshots[0].State != "SUCCESS" || snapshots[0].Name != "snapshot1" ||
		snapshots[0].Shards.Successful != 93 {
		t.Errorf("Unexpected snapshots, got %v", snapshots)
	}

	if snapshots[0].Indices[0] != "index1" || snapshots[0].Indices[1] != "index2" ||
		len(snapshots[0].Indices) != 2 {
		t.Errorf("Unexpected snapshots, got %v", snapshots)
	}
}

func TestGetSnapshots_Inprogress(t *testing.T) {
	testSetup := &ServerSetup{
		Method: "GET",
		Path:   "/_snapshot/octocat/_all",
		Response: `{
  "snapshots": [
    {
      "snapshot": "snapshot1",
      "uuid": "kXx-r58tSOeVvDbvCC1IsQ",
      "version_id": 5060699,
      "version": "5.6.6",
      "indices": [ "index1", "index2" ],
      "state": "IN_PROGRESS",
      "start_time": "2018-04-03T06:06:24.837Z",
      "start_time_in_millis": 1522735584837,
      "duration_in_millis": 3600000,
      "failures": [],
      "shards": { "total": 93, "failed": 0, "successful": 93 }
    }
  ]
}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	snapshots, err := client.GetSnapshots("octocat")
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("Unexpected snapshots, got %v", snapshots)
	}

	if snapshots[0].State != "IN_PROGRESS" {
		t.Errorf("Unexpected snapshots, got %v", snapshots)
	}
}

func TestGetSnapshotStatus(t *testing.T) {
	testSetup := &ServerSetup{
		Method: "GET",
		Path:   "/_snapshot/octocat/snapshot1",
		Response: `{
  "snapshots": [
    {
      "snapshot": "snapshot1",
      "uuid": "kXx-r58tSOeVvDbvCC1IsQ",
      "version_id": 5060699,
      "version": "5.6.6",
      "indices": [ "index1", "index2" ],
      "state": "SUCCESS",
      "start_time": "2018-04-03T06:06:24.837Z",
      "start_time_in_millis": 1522735584837,
      "end_time": "2018-04-03T07:41:01.719Z",
      "end_time_in_millis": 1522741261719,
      "duration_in_millis": 1000,
      "failures": [],
      "shards": { "total": 93, "failed": 0, "successful": 93 }
    }
  ]
}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	snapshot, err := client.GetSnapshotStatus("octocat", "snapshot1")
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if snapshot.State != "SUCCESS" {
		t.Errorf("Unexpected state, got %+v", snapshot)
	}

	if snapshot.Name != "snapshot1" {
		t.Errorf("Unexpected name, got %+v", snapshot)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "DELETE",
		Path:     "/_snapshot/octocat/snapshot1",
		Response: `{"acknowledged": true}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	err := client.DeleteSnapshot("octocat", "snapshot1")
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}
}

func TestVerifyRepository(t *testing.T) {
	testSetup := &ServerSetup{
		Method:   "POST",
		Path:     "/_snapshot/octocat/_verify",
		Response: `{"nodes":{"YaTBa_BtRmOoz1bHKJeQ8w":{"name":"YaTBa_B"}}}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	verified, err := client.VerifyRepository("octocat")
	if err != nil {
		t.Errorf("Unexpected error, got %s", err)
	}

	if !verified {
		t.Errorf("Expected repository to be verified, got %v", verified)
	}
}

func TestListRepositories(t *testing.T) {
	testSetup := &ServerSetup{
		Method: "GET",
		Path:   "/_snapshot/_all",
		Response: `{
			"fileSystemRepo": { "type": "fs", "settings": { "location": "/foo/bar" } },
			"s3Repo": { "type": "s3", "settings": { "bucket": "myBucket", "base_path": "foo", "access_key": "access", "secret_key": "secret" } }
}`,
	}

	host, port, ts := setupTestServers(t, []*ServerSetup{testSetup})
	defer ts.Close()
	client := NewClient(host, port)

	repos, err := client.ListRepositories()

	if err != nil {
		t.Fatalf("Got error getting repositories: %s", err)
	}

	if len(repos) != 2 {
		t.Fatalf("Expected two repositories, got %d", len(repos))
	}

	fsRepo := repos[0]

	if fsRepo.Name != "fileSystemRepo" || fsRepo.Type != "fs" || fsRepo.Settings["location"] != "/foo/bar" {
		t.Fatalf("Unexpected fs repo settings, got: %+v", fsRepo)
	}

	s3Repo := repos[1]

	if s3Repo.Name != "s3Repo" || s3Repo.Type != "s3" || s3Repo.Settings["bucket"] != "myBucket" {
		t.Fatalf("Unexpected s3 repo settings, got: %+v", s3Repo)
	}

	if _, exists := s3Repo.Settings["access_key"]; exists {
		t.Fatalf("Expected access_key to be scrubbed from s3Repo.")
	}

	if _, exists := s3Repo.Settings["secret_key"]; exists {
		t.Fatalf("Expected secret_key to be scrubbed from s3Repo.")
	}
}
