// main.go
package main

import (
	"fmt"
	"os"
	"regexp"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var (
	workloadRoleMemberRe = regexp.MustCompile(`serviceAccount:(.+)\.svc.(.*)`)
	gcpRoleMemberRe      = regexp.MustCompile(`(.*)@(.*)\.iam\.gserviceaccount\.com`)
)

func main() {
	// create a struct matching the structure of ResourceList.FunctionConfig to hold its data
	var config struct {
		Data map[string]string `yaml:"data"`
	}
	fn := func(items []*yaml.RNode) ([]*yaml.RNode, error) {
		projectID, found := config.Data["projectID"]
		if !found {
			return nil, fmt.Errorf("must specify projectID")
		}
		for i := range items {
			if err := processResource(items[i], projectID); err != nil {
				return nil, err
			}
		}
		return items, nil
	}
	p := framework.SimpleProcessor{Filter: kio.FilterFunc(fn), Config: &config}
	cmd := command.Build(p, command.StandaloneDisabled, false)
	// Adds a "gen" subcommand to create a Dockerfile for building the function into a container image.
	command.AddGenerateDockerfile(cmd)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func processResource(r *yaml.RNode, projectID string) error {
	meta, err := r.GetMeta()
	if err != nil {
		return err
	}

	if meta.Kind != "IAMPolicyMember" {
		return nil
	}
	// extract spec.role
	specNode := r.Field("spec")
	if specNode.IsNilOrEmpty() {
		return nil
	}

	role := yaml.GetValue(specNode.Value.Field("role").Value)
	member := yaml.GetValue(specNode.Value.Field("member").Value)
	switch role {
	case "roles/iam.workloadIdentityUser":
		matches := workloadRoleMemberRe.FindSubmatch([]byte(member))
		memberValue := fmt.Sprintf("serviceAccount:%s.svc.%s", projectID, matches[2])
		_, err = r.Pipe(
			yaml.PathGetter{Path: []string{"spec"}},
			yaml.FieldSetter{Name: "member", Value: yaml.NewStringRNode(memberValue)})
		return err
	case "roles/source.reader":
		matches := gcpRoleMemberRe.FindSubmatch([]byte(member))
		memberValue := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", matches[1], projectID)
		_, err = r.Pipe(
			yaml.PathGetter{Path: []string{"spec"}},
			yaml.FieldSetter{Name: "member", Value: yaml.NewStringRNode(memberValue)})
		if err != nil {
			return err
		}
		_, err = r.Pipe(
			yaml.PathGetter{Path: []string{"spec", "resourceRef"}},
			yaml.FieldSetter{
				Name:  "external",
				Value: yaml.NewStringRNode(fmt.Sprintf("projects/%s", projectID)),
			})
		return err
	default:
		return nil
	}
}
