package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	serfclient "github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	glog "github.com/magicsong/color-glog"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
)

type Config struct {
	agent.Config
	SerfRPCAddress string
	SerfRPCAuthKey string
	MemberChan     chan *[]serfclient.Member
}

type Cluster struct {
	config *Config
	client *serfclient.RPCClient
}

var mediaFilter = map[string]string{"node": "media"}

// Create a connection to a new Cluster that will immediately connect
func NewCluster(config *Config) *Cluster {
	c := Cluster{
		config: config,
	}
	return &c
}

// Start the connection to this cluster. Blocks until error.
// TODO: Is this a good pattern for doing this?
func (c *Cluster) Start() error {
	errchan := make(chan error)

	// client
	go func() {
		// in a loop in case client boots before server
		for {
			err := c.runClient()
			if err != nil {
				glog.Errorf("Error starting client: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}
			// nil error means we're shutting down
			glog.Infof("Shutting down on Serf client failure")
			errchan <- err
		}
	}()

	// server
	go func() {
		err := c.runServer()
		errchan <- err
	}()

	return <-errchan
}

func (c *Cluster) MembersFiltered(filter map[string]string, status, name string) ([]serfclient.Member, error) {
	return c.client.MembersFiltered(filter, status, name)
}

func (c *Cluster) Member(filter map[string]string, status, name string) (*serfclient.Member, error) {
	members, err := c.MembersFiltered(filter, status, name)
	if err != nil {
		return nil, err
	}
	if len(members) < 1 {
		return nil, fmt.Errorf("could not find serf member name=%s", name)
	}
	if len(members) > 1 {
		glog.Errorf("found multiple serf members with the same name! this shouldn't happen! name=%s count=%d", name, len(members))
	}
	return &members[0], nil
}

// var getSerfMember = member

func (c *Cluster) runServer() error {
	// Everything past this is booting up Serf
	tmpFile, err := c.writeSerfConfig()
	if err != nil {
		glog.Fatalf("Error writing serf config: %s", err)
	}
	defer os.Remove(tmpFile)
	glog.V(6).Infof("Wrote serf config to %s", tmpFile)

	ui := &cli.BasicUi{Writer: os.Stdout}

	// copied from:
	// https://github.com/hashicorp/serf/blob/a2bba5676d6e37953715ea10e583843793a0c507/cmd/serf/commands.go#L20-L25
	// we should consider invoking serf directly instead of wrapping their CLI helper
	commands := map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			a := &agent.Command{
				Ui:         ui,
				ShutdownCh: make(chan struct{}),
			}
			return a, nil
		},
	}

	cli := &cli.CLI{
		Args:     []string{"agent", "-config-file", tmpFile},
		Commands: commands,
		HelpFunc: cli.BasicHelpFunc("catalyst-node"),
	}

	exitCode, err := cli.Run()
	if err != nil {
		return err
	}

	return fmt.Errorf("serf exited code=%d", exitCode)
}

func (c *Cluster) connectSerfAgent(serfRPCAddress, serfRPCAuthKey string) (*serfclient.RPCClient, error) {
	return serfclient.ClientFromConfig(&serfclient.Config{
		Addr:    serfRPCAddress,
		AuthKey: serfRPCAuthKey,
	})
}

func (c *Cluster) runClient() error {
	client, err := c.connectSerfAgent(c.config.SerfRPCAddress, c.config.SerfRPCAuthKey)

	if err != nil {
		return err
	}
	c.client = client
	defer client.Close()

	eventCh := make(chan map[string]interface{})
	streamHandle, err := client.Stream("*", eventCh)
	if err != nil {
		return fmt.Errorf("error starting stream: %w", err)
	}
	defer client.Stop(streamHandle)

	inbox := make(chan map[string]interface{}, 1)
	go func() {
		for {
			e := <-eventCh
			select {
			case inbox <- e:
				// Event is now in the inbox
			default:
				// Overflow event gets dropped
			}
		}
	}()

	// Ping the inbox initially and then every few seconds to retry on the load balancer
	go func() {
		for {
			e := map[string]interface{}{}
			select {
			case inbox <- e:
			default:
			}
			time.Sleep(5 * time.Second)
		}
	}()

	for {
		event := <-inbox
		glog.V(5).Infof("got event: %v", event)

		members, err := c.client.MembersFiltered(mediaFilter, ".*", ".*")

		if err != nil {
			glog.Errorf("Error getting serf, crashing: %v\n", err)
			break
		}

		c.config.MemberChan <- &members
		continue
	}

	return nil
}

func (c *Cluster) writeSerfConfig() (string, error) {
	serfConfig := &agent.Config{
		BindAddr:      c.config.BindAddr,
		AdvertiseAddr: c.config.AdvertiseAddr,
		RPCAddr:       c.config.RPCAddr,
		EncryptKey:    c.config.EncryptKey,
		Profile:       c.config.Profile,
		NodeName:      c.config.NodeName,
		RetryJoin:     c.config.RetryJoin,
		Tags:          c.config.Tags,
	}

	// Two steps to properly serialize this as JSON: https://github.com/spf13/viper/issues/816#issuecomment-1149732004
	items := map[string]interface{}{}
	if err := mapstructure.Decode(serfConfig, &items); err != nil {
		return "", err
	}
	b, err := json.Marshal(items)

	if err != nil {
		return "", err
	}

	// Everything after this is booting up serf with our provided config flags:
	tmpFile, err := ioutil.TempFile(os.TempDir(), "serf-config-*.json")
	if err != nil {
		return "", err
	}

	// Example writing to the file
	if _, err = tmpFile.Write(b); err != nil {
		return "", err
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return tmpFile.Name(), err
}
