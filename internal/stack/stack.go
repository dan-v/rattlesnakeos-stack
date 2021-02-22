package stack

import (
	"context"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	DefaultDeployTimeout          = time.Minute * 5
)

type TemplateRenderer interface {
	RenderAll() error
}

type CloudSetup interface {
	Setup(ctx context.Context) error
}

type CloudSubscriber interface {
	Subscribe(ctx context.Context) (bool, error)
}

type TerraformApplier interface {
	Apply(ctx context.Context) ([]byte, error)
}

type Stack struct {
	name              string
	templateRenderer  TemplateRenderer
	cloudSetup        CloudSetup
	cloudSubscriber   CloudSubscriber
	terraformApplier  TerraformApplier
}

func New(name string, templateRenderer TemplateRenderer, cloudSetup CloudSetup, cloudSubscriber CloudSubscriber, terraformApplier TerraformApplier) *Stack {
	return &Stack{
		name:            	name,
		templateRenderer:  	templateRenderer,
		cloudSetup:     	cloudSetup,
		cloudSubscriber: 	cloudSubscriber,
		terraformApplier: 	terraformApplier,
	}
}

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
	if subscribed, err := s.cloudSubscriber.Subscribe(ctx); err != nil {
		return err
	} else {
		if subscribed {
			log.Infof("Successfully setup email notifications for stack %v - you'll need to click link in " +
				"confirmation email to get notifications.", s.name)
		}
	}

	log.Infof("Successfully deployed/updated resources for stack %v", s.name)
	return nil
}
