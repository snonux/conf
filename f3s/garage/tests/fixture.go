// fixture records the real recipe with dummy credentials and a generic host.
// It never applies a plan or reads a real secret provider.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/snonux/conf/gonf/garage"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/plan"
	"github.com/snonux/gonf/secret"
)

type input struct {
	Metrics string
	Mode    string
	Admin   string
	RPC     string
	Render  bool
}

func main() {
	if err := record(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func record() error {
	var in input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		return err
	}
	SetSecretProvider(secret.ProviderFunc(func(_ context.Context, ref secret.Ref) ([]byte, error) {
		if strings.HasSuffix(string(ref), "rpc_secret") {
			return []byte(strings.Repeat("ab", 32)), nil
		}
		if in.Mode == "missing" {
			return nil, &secret.Error{Kind: secret.ErrNotFound, Ref: ref, Msg: "fixture absent"}
		}
		if in.Mode == "failure" {
			return nil, fmt.Errorf("fixture unavailable")
		}
		return []byte(in.Metrics), nil
	}))
	Cluster("fixture", Host("a.example", WithPlatform("freebsd/amd64"),
		WithData(garage.Node{RPCPublicAddr: in.RPC, AdminAPIBindAddr: in.Admin})))
	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), OnCluster("fixture"))
	ops, err := RecordPlan("fixture", "", "garage_config")
	if err != nil {
		return err
	}
	for _, op := range ops {
		if !in.Render {
			if err := json.NewEncoder(os.Stdout).Encode(op); err != nil {
				return err
			}
			continue
		}
		payload, ok := op.Payload.(plan.FilePayload)
		if !ok {
			continue
		}
		source, err := base64.StdEncoding.DecodeString(payload.ContentB64)
		if err != nil {
			return err
		}
		var data map[string]any
		if err := json.Unmarshal(payload.TemplateData, &data); err != nil {
			return err
		}
		t, err := template.New("garage").Option("missingkey=error").Parse(string(source))
		if err != nil {
			return err
		}
		if err := t.Execute(os.Stdout, data); err != nil {
			return err
		}
	}
	return nil
}
