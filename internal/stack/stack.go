package stack

import (
	log "github.com/sirupsen/logrus"
)

const (
	MinimumChromiumVersion        = 86
	DefaultCoreConfigRepo         = "https://github.com/rattlesnakeos/core"
	DefaultLatestURL              = "https://raw.githubusercontent.com/RattlesnakeOS/latest/11.0/latest.json"
	RattlesnakeOSStackReleasesURL = "https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
)

type TemplateGenerator interface {
	RenderAll() error
}

type CloudClient interface {
	Setup() error
	Subscribe() error
}

type TerraformClient interface {
	Apply() ([]byte, error)
	Destroy() ([]byte, error)
}

type Stack struct {
	name              string
	templates         TemplateGenerator
	cloudClient       CloudClient
	terraformClient   TerraformClient
	outputDirFullPath string
	tfDir             string
}

func New(name string, templates TemplateGenerator, cloudClient CloudClient, terraformClient TerraformClient) *Stack {
	return &Stack{
		name:            name,
		templates:       templates,
		cloudClient:     cloudClient,
		terraformClient: terraformClient,
	}
}

func (s *Stack) Apply() error {
	log.Infof("Rendering all templates files for stack %v", s.name)
	if err := s.templates.RenderAll(); err != nil {
		return err
	}

	log.Infof("Creating/updating non Terraform AWS resources for stack %v", s.name)
	if err := s.cloudClient.Setup(); err != nil {
		return err
	}

	log.Infof("Executing Terraform apply for stack %v", s.name)
	if _, err := s.terraformClient.Apply(); err != nil {
		return err
	}

	if err := s.cloudClient.Subscribe(); err != nil {
		return err
	}

	log.Infof("Successfully deployed/updated AWS resources for stack %v", s.name)
	return nil
}

func (s *Stack) Destroy() error {
	log.Info("Executing Terraform destroy for stack %v", s.name)
	if _, err := s.terraformClient.Destroy(); err != nil {
		return err
	}

	log.Infof("Successfully removed AWS resources for stack %v", s.name)
	return nil
}
