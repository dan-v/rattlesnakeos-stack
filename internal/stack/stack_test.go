package stack_test

import (
	"context"
	"errors"
	"github.com/dan-v/rattlesnakeos-stack/internal/stack"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	errTemplateRender = errors.New("template renderer error")
	errCloudSetup     = errors.New("cloud setup error")
	errCloudSubscribe = errors.New("cloud subscriber error")
	errTerraformApply = errors.New("terraform apply error")
)

func TestDeploy(t *testing.T) {
	tests := map[string]struct {
		stack    *stack.Stack
		expected error
	}{
		"deploy with no errors and subscribe true": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: nil},
				&fakeCloudSetup{err: nil},
				&fakeCloudSubscriber{subscribed: true, err: nil},
				&fakeTerraformApplier{output: []byte("test"), err: nil},
			),
			expected: nil,
		},
		"deploy with no errors and subscribe false": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: nil},
				&fakeCloudSetup{err: nil},
				&fakeCloudSubscriber{subscribed: false, err: nil},
				&fakeTerraformApplier{output: []byte("test"), err: nil},
			),
			expected: nil,
		},
		"template render error": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: errTemplateRender},
				&fakeCloudSetup{err: nil},
				&fakeCloudSubscriber{subscribed: false, err: nil},
				&fakeTerraformApplier{output: []byte("test"), err: nil},
			),
			expected: errTemplateRender,
		},
		"cloud setup error": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: nil},
				&fakeCloudSetup{err: errCloudSetup},
				&fakeCloudSubscriber{subscribed: false, err: nil},
				&fakeTerraformApplier{output: []byte("test"), err: nil},
			),
			expected: errCloudSetup,
		},
		"cloud subscribe error": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: nil},
				&fakeCloudSetup{err: nil},
				&fakeCloudSubscriber{subscribed: false, err: errCloudSubscribe},
				&fakeTerraformApplier{output: []byte("test"), err: nil},
			),
			expected: errCloudSubscribe,
		},
		"terraform apply error": {
			stack: stack.New(
				"test",
				&fakeTemplateRenderer{err: nil},
				&fakeCloudSetup{err: nil},
				&fakeCloudSubscriber{subscribed: false, err: nil},
				&fakeTerraformApplier{output: []byte("test"), err: errTerraformApply},
			),
			expected: errTerraformApply,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.stack.Deploy(context.Background())
			assert.ErrorIs(t, err, tc.expected)
		})
	}
}

type fakeTemplateRenderer struct {
	err error
}

func (f *fakeTemplateRenderer) RenderAll() error {
	return f.err
}

type fakeCloudSetup struct {
	err error
}

func (f *fakeCloudSetup) Setup(ctx context.Context) error {
	return f.err
}

type fakeCloudSubscriber struct {
	subscribed bool
	err        error
}

func (f *fakeCloudSubscriber) Subscribe(ctx context.Context) (bool, error) {
	return f.subscribed, f.err
}

type fakeTerraformApplier struct {
	output []byte
	err    error
}

func (f *fakeTerraformApplier) Apply(ctx context.Context) ([]byte, error) {
	return f.output, f.err
}
