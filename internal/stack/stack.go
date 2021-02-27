package stack

import (
	"context"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	// DefaultDeployTimeout is the default timeout for deployments
	DefaultDeployTimeout = time.Minute * 5
)

// TemplateRenderer is an interface for template rendering
type TemplateRenderer interface {
	RenderAll() error
}

// CloudSetup is an interface for cloud setup
type CloudSetup interface {
	Setup(ctx context.Context) error
}

// CloudSubscriber is an interface for cloud subscription
type CloudSubscriber interface {
	Subscribe(ctx context.Context) (bool, error)
}

// TerraformApplier is an interface for applying terraform
type TerraformApplier interface {
	Apply(ctx context.Context) ([]byte, error)
}

// Stack contains all the necessary pieces to generate and deploy a stack
type Stack struct {
	name             string
	templateRenderer TemplateRenderer
	cloudSetup       CloudSetup
	cloudSubscriber  CloudSubscriber
	terraformApplier TerraformApplier
}

// New returns an initialized Stack that is ready for deployment
func New(name string, templateRenderer TemplateRenderer, cloudSetup CloudSetup, cloudSubscriber CloudSubscriber, terraformApplier TerraformApplier) *Stack {
	return &Stack{
		name:             name,
		templateRenderer: templateRenderer,
		cloudSetup:       cloudSetup,
		cloudSubscriber:  cloudSubscriber,
		terraformApplier: terraformApplier,
	}
}

// Deploy renders files, runs cloud setup, runs terraform apply, and ensures notifications are setup
func (s *Stack) Deploy(ctx context.Context) error {
	log.Infof("Rendering all templates files for stack %v", s.name)
	if err := s.templateRenderer.RenderAll(); err != nil {
		return err
	}

	log.Infof("Creating/updating non terraform resources for stack %v", s.name)
	if err := s.cloudSetup.Setup(ctx); err != nil {
		return err
	}

	log.Infof("Executing terraform apply for stack %v", s.name)
	if _, err := s.terraformApplier.Apply(ctx); err != nil {
		return err
	}

	log.Infof("Ensuring notifications enabled for stack %v", s.name)
	subscribed, err := s.cloudSubscriber.Subscribe(ctx)
	if err != nil {
		return err
	}
	if subscribed {
		log.Infof("Successfully setup email notifications for stack %v - you'll need to click link in "+
			"confirmation email to get notifications.", s.name)
	}

	log.Infof("Successfully deployed/updated resources for stack %v", s.name)
	return nil
}
