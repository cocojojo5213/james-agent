package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cocojojo5213/james-agent/internal/bus"
	"github.com/cocojojo5213/james-agent/internal/config"
)

type ChannelManager struct {
	channels map[string]Channel
	bus      *bus.MessageBus
}

func NewChannelManager(cfg config.ChannelsConfig, b *bus.MessageBus) (*ChannelManager, error) {
	m := &ChannelManager{
		channels: make(map[string]Channel),
		bus:      b,
	}

	if cfg.Telegram.Enabled {
		ch, err := NewTelegramChannel(cfg.Telegram, b)
		if err != nil {
			return nil, fmt.Errorf("init telegram channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(msg); err != nil {
				slog.Error("send failed", "channel", ch.Name(), "error", err)
			}
		})
	}

	if cfg.Feishu.Enabled {
		ch, err := NewFeishuChannel(cfg.Feishu, b)
		if err != nil {
			return nil, fmt.Errorf("init feishu channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(msg); err != nil {
				slog.Error("send failed", "channel", ch.Name(), "error", err)
			}
		})
	}

	if cfg.WeCom.Enabled {
		ch, err := NewWeComChannel(cfg.WeCom, b)
		if err != nil {
			return nil, fmt.Errorf("init wecom channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(msg); err != nil {
				slog.Error("send failed", "channel", ch.Name(), "error", err)
			}
		})
	}

	if cfg.WhatsApp.Enabled {
		ch, err := NewWhatsApp(cfg.WhatsApp, b)
		if err != nil {
			return nil, fmt.Errorf("create whatsapp channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(msg); err != nil {
				slog.Error("send failed", "channel", ch.Name(), "error", err)
			}
		})
	}

	return m, nil
}

func NewChannelManagerWithGateway(cfg config.ChannelsConfig, gwCfg config.GatewayConfig, b *bus.MessageBus) (*ChannelManager, error) {
	m, err := NewChannelManager(cfg, b)
	if err != nil {
		return nil, err
	}

	if cfg.WebUI.Enabled {
		ch, err := NewWebUIChannel(cfg.WebUI, gwCfg, b)
		if err != nil {
			return nil, fmt.Errorf("init webui channel: %w", err)
		}
		m.channels[ch.Name()] = ch
		b.SubscribeOutbound(ch.Name(), func(msg bus.OutboundMessage) {
			if err := ch.Send(msg); err != nil {
				slog.Error("send failed", "channel", ch.Name(), "error", err)
			}
		})
	}

	return m, nil
}

func (m *ChannelManager) StartAll(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(m.channels))

	for name, ch := range m.channels {
		wg.Add(1)
		go func(name string, ch Channel) {
			defer wg.Done()
			slog.Info("starting channel", "channel", name)
			if err := ch.Start(ctx); err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
			}
		}(name, ch)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		return err
	}
	return nil
}

func (m *ChannelManager) StopAll() error {
	for name, ch := range m.channels {
		slog.Info("stopping channel", "channel", name)
		if err := ch.Stop(); err != nil {
			slog.Error("error stopping channel", "channel", name, "error", err)
		}
	}
	return nil
}

func (m *ChannelManager) EnabledChannels() []string {
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}
