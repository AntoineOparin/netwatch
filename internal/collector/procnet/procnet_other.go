//go:build !darwin

package procnet

import (
	"context"

	"github.com/AntoineOparin/netwatch/internal/collector"
)

func Supported() bool { return false }

type Collector struct{}

func New() *Collector { return &Collector{} }

func (c *Collector) Name() string { return "procnet" }

func (c *Collector) Run(context.Context, chan<- collector.Sample) error {
	return ErrUnsupported
}
