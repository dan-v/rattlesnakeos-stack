package stack

import (
	"context"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	MinimumChromiumVersion        = 86
	DefaultDeployTimeout          = time.Minute * 5
	DefaultCoreConfigRepo         = "https://github.com/rattlesnakeos/core"
	DefaultLatestURL              = "https://raw.githubusercontent.com/RattlesnakeOS/latest/11.0/latest.json"
	DefaultReleaseURL             = "https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
)

type TemplateRenderer interface {
	RenderAll() error
}

type CloudSetupSubscriber interface {
	Setup(ctx context.Context) error
	SubscribeNotifications(ctx context.Context) error
}

type TerraformApplier interface {
	Apply(ctx context.Context) ([]byte, error)
}

type Stack struct {
	name              string
	templateClient    TemplateRenderer
	cloudClient       CloudSetupSubscriber
	terraformClient   TerraformApplier
}

func New(name string, templateClient TemplateRenderer, cloudClient CloudSetupSubscriber, terraformClient TerraformApplier) *Stack {
	return &Stack{
		name:            name,
		templateClient:  templateClient,
		cloudClient:     cloudClient,
		terraformClient: terraformClient,
	}
}

func (s *Stack) Deploy(ctx context.Context) error {
	log.Infof("Rendering all templates files for stack %v", s.name)
	if err := s.templateClient.RenderAll(); err != nil {
		return err
	}

	log.Infof("Creating/updating non terraform resources for stack %v", s.name)
	if err := s.cloudClient.Setup(ctx); err != nil {
		return err
	}

	log.Infof("Executing terraform apply for stack %v", s.name)
	if _, err := s.terraformClient.Apply(ctx); err != nil {
		return err
	}

	log.Infof("Ensuring notifications enabled for stack %v", s.name)
	if err := s.cloudClient.SubscribeNotifications(ctx); err != nil {
		return err
	}

	log.Infof("Successfully deployed/updated resources for stack %v", s.name)
	return nil
}
