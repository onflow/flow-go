package testingdock_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	"github.com/m4ksio/testingdock"
)

const (
	flowIntegrationTest       = "FLOW_INTEGRATION_TEST"
	flowEnableIntegrationTest = "on"
)

func TestContainer_Start(t *testing.T) {

	if os.Getenv(flowIntegrationTest) != flowEnableIntegrationTest {
		t.Skipf("Integration tests not enabled, set %s system variable to '%s'", flowIntegrationTest, flowEnableIntegrationTest)
	}

	// create suite

	testingdock.Verbose = true

	name := "mvp"
	suite, ok := testingdock.GetOrCreateSuite(t, name, testingdock.SuiteOpts{})
	if ok {
		t.Fatal("this suite should not exists yet")
	}

	// create network
	n := suite.Network(testingdock.NetworkOpts{
		Name: name,
	})

	collectionNodeApi := testingdock.RandomPort(t)

	collectionNode := suite.Container(testingdock.ContainerOpts{
		Name:      "collection",
		ForcePull: false,
		Config: &container.Config{
			Image: "gcr.io/dl-flow/collection:latest",
			Cmd:   []string{"--entries=collection-c3bb99b34fdbd57eae8378a317af29b4b2da07610fd3a866c3a5ce9a19173295@collection:9000=10000"},
		},
		HostConfig: &container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port("9000/tcp"): []nat.PortBinding{
					{
						HostPort: collectionNodeApi,
					},
				},
			},
		},
		//HealthCheck: testingdock.HealthCheckCustom(db.Ping),
		//Reset: testingdock.ResetCustom(func() error {
		//	_, err := db.Exec(`
		//		DROP SCHEMA public CASCADE;
		//		DROP SCHEMA mnemosyne CASCADE;
		//		CREATE SCHEMA public;
		//	`)
		//	return err
		//}),
	})

	// add postgres to the test network
	n.After(collectionNode)
	// start mnemosyned after postgres, this also adds it to the test network
	//postgres.After(mnemosyned)

	// start the network, this also starts the containers
	suite.Start(context.Background())
	defer suite.Close()

	//testQueries(t, db)

}

func testQueries(t *testing.T, db *sql.DB) {
	_, err := db.ExecContext(context.TODO(), "CREATE TABLE public.example (name TEXT);")
	if err != nil {
		t.Fatalf("table creation error: %s", err.Error())
	}
	_, err = db.ExecContext(context.TODO(), "INSERT INTO public.example (name) VALUES ('anything')")
	if err != nil {
		t.Fatalf("insert error: %s", err.Error())
	}
	_, err = db.ExecContext(context.TODO(), "INSERT INTO mnemosyne.session (access_token, refresh_token,subject_id, bag) VALUES ('123', '123', '1', '{}')")
	if err != nil {
		t.Fatalf("insert error: %s", err.Error())
	}
}
