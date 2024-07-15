package config

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/layou233/zbproxy/v3/common/set"

	"github.com/fsnotify/fsnotify"
	"github.com/phuslu/log"
)

type _Root struct {
	Log       Log
	Services  []*Service
	Router    Router
	Outbounds []*Outbound
	Lists     map[string]set.StringSet
}

type Root struct {
	Log       Log
	Services  []*Service
	Router    Router
	Outbounds []*Outbound
	Lists     map[string]set.StringSet

	ctx           context.Context
	logger        *log.Logger
	filePath      string
	watcher       *fsnotify.Watcher
	reloadChan    chan struct{}
	updateHandler func()
}

func (r *Root) WatcherEnabled() bool {
	return r.watcher != nil
}

// SetUpdateHandler sets a function that would be called
// after the config reloading.
func (r *Root) SetUpdateHandler(handler func()) {
	r.updateHandler = handler
}

// Reload tries to reload the config and returns false if another reloading is on the way.
// Only takes effect when watcher is enabled.
func (r *Root) Reload() bool {
	if r.reloadChan == nil {
		return false
	}
	select {
	case r.reloadChan <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *Root) Close() (err error) {
	if r.watcher != nil {
		close(r.reloadChan)
		err = r.watcher.Close()
		r.watcher = nil
	}
	return
}

func (r *Root) reloadEventLoop() {
	for {
		select {
		case _, ok := <-r.reloadChan:
			if !ok {
				return
			}
			r.logger.Debug().Msg("Config reload triggered manually")
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			r.logger.Debug().Uint32("operation", uint32(event.Op)).Msg("Config update detected")
		case err, ok := <-r.watcher.Errors:
			if ok {
				r.logger.Debug().Err(err).Msg("Error when listening reload events")
			}
			return
		case <-r.ctx.Done():
			r.logger.Debug().Err(r.ctx.Err()).Msg("Closing config watcher")
			r.Close()
			return
		}
		startTime := time.Now()

		var rawConfig _Root
		err := loadContent(&rawConfig, r.filePath)
		if err != nil {
			r.logger.Error().Err(err).Msg("Error when loading content from file")
			continue
		}

		r.Log = rawConfig.Log
		r.Services = rawConfig.Services
		r.Router = rawConfig.Router
		r.Outbounds = rawConfig.Outbounds
		r.Lists = rawConfig.Lists

		if r.updateHandler != nil {
			r.updateHandler()
		}
		r.logger.Info().Str("duration", time.Now().Sub(startTime).String()).Msg("Config reloaded successfully")
	}
}

func loadContent(root *_Root, filePath string) error {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(fileContent, root)
	if err != nil {
		return err
	}
	return nil
}

func LoadConfigFromFile(ctx context.Context, filePath string, watch bool, logger *log.Logger) (*Root, error) {
	var rawConfig _Root
	err := loadContent(&rawConfig, filePath)
	if err != nil {
		return nil, err
	}
	root := &Root{
		Log:       rawConfig.Log,
		Services:  rawConfig.Services,
		Router:    rawConfig.Router,
		Outbounds: rawConfig.Outbounds,
		Lists:     rawConfig.Lists,
		ctx:       ctx,
		logger:    logger,
		filePath:  filePath,
	}
	if watch {
		root.watcher, err = fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		err = root.watcher.Add(filePath)
		if err != nil {
			root.Close()
			return nil, err
		}
		root.reloadChan = make(chan struct{})
		go root.reloadEventLoop()
	}
	return root, nil
}