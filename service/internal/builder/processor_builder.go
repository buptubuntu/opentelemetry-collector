package builder

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type builtProcessor struct {
	dataType config.DataType
	cfg      config.Processor
	factory  component.ProcessorFactory
	set      component.ProcessorCreateSettings

	tc consumer.Traces
	mc consumer.Metrics
	lc consumer.Logs

	nextTc *builtProcessor
	nextMc *builtProcessor
	nextLc *builtProcessor

	mutatesData bool
	processor   component.Processor
	id          config.ComponentID
}

type builtProcessors map[int]*builtProcessor

func (btProc *builtProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return btProc.tc.ConsumeTraces(ctx, td)
}

func (btProc *builtProcessor) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return btProc.mc.ConsumeMetrics(ctx, md)
}

func (btProc *builtProcessor) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	return btProc.lc.ConsumeLogs(ctx, ld)
}

func (btProc *builtProcessor) Start(ctx context.Context, host component.Host) error {
	return btProc.processor.Start(ctx, host)
}

func (btProc *builtProcessor) Shutdown(ctx context.Context) error {
	return btProc.processor.Shutdown(ctx)
}

func (btProc *builtProcessor) Capabilities() consumer.Capabilities {
	switch btProc.dataType {
	case config.TracesDataType:
		return btProc.tc.Capabilities()
	case config.MetricsDataType:
		return btProc.mc.Capabilities()
	case config.LogsDataType:
		return btProc.lc.Capabilities()
	}
	return consumer.Capabilities{MutatesData: btProc.mutatesData}
}

func (btProc *builtProcessor) Reload(host component.Host, ctx context.Context, cfg interface{}) error {
	procCfg, ok := cfg.(config.Processor)
	if !ok {
		return fmt.Errorf("error when reaload processor:%q for invalid config:%v", btProc.id, cfg)
	}

	if procCfg.ID() != btProc.id {
		return fmt.Errorf("error when reload processor:%q for invalid conf id:%v", btProc.id, procCfg.ID())
	}

	// TODO compare config to decide if reload is needed

	if reloadableProc, ok := btProc.processor.(component.Reloadable); ok {
		return reloadableProc.Reload(host, ctx, cfg)
	}

	oldProcessor := btProc.processor

	var err error
	switch btProc.dataType {
	case config.TracesDataType:
		var proc component.TracesProcessor
		proc, err = btProc.factory.CreateTracesProcessor(ctx, btProc.set, procCfg, btProc.nextTc)

		if proc != nil && err == nil {
			err = proc.Start(ctx, host)
		}

		if proc == nil {
			return fmt.Errorf("factory produce nil processor for component id:%q inputtype:%v config:%v during reload", procCfg.ID(), btProc.dataType, procCfg)
		}

		if proc != nil && err == nil {
			btProc.mutatesData = proc.Capabilities().MutatesData
			btProc.processor = proc
			btProc.tc = proc
		}
	case config.MetricsDataType:
		var proc component.MetricsProcessor
		proc, err = btProc.factory.CreateMetricsProcessor(ctx, btProc.set, procCfg, btProc.nextMc)

		if proc != nil && err == nil {
			err = proc.Start(ctx, host)
		}

		if proc == nil {
			return fmt.Errorf("factory produce nil processor for component id:%q inputtype:%v config:%v during reload", procCfg.ID(), btProc.dataType, procCfg)
		}

		if proc != nil && err == nil {
			btProc.mutatesData = proc.Capabilities().MutatesData
			btProc.processor = proc
			btProc.mc = proc
		}
	case config.LogsDataType:
		var proc component.LogsProcessor
		proc, err = btProc.factory.CreateLogsProcessor(ctx, btProc.set, procCfg, btProc.nextLc)

		if proc != nil && err == nil {
			err = proc.Start(ctx, host)
		}

		if proc == nil {
			return fmt.Errorf("factory produce nil processor for component id:%q inputtype:%v config:%v during reload", procCfg.ID(), btProc.dataType, procCfg)
		}

		if proc != nil && err == nil {
			btProc.mutatesData = proc.Capabilities().MutatesData
			btProc.processor = proc
			btProc.lc = proc
		}
	}

	if err != nil {
		return fmt.Errorf("error creating processor %q: during reload:%v", procCfg.ID(), err)
	}

	if err := oldProcessor.Shutdown(ctx); err != nil {
		return fmt.Errorf("error when shutdown old processor:%q during reload:%v", btProc.id, err)
	}

	return nil
}

type processorsBuilder struct {
	set       component.ProcessorCreateSettings
	config    *config.Config
	factories map[config.Type]component.ProcessorFactory
}

func (procBuilder *processorsBuilder) buildProcessors(ctx context.Context, dataType config.DataType, processors []config.ComponentID) (builtProcessors, error) {
	var err error
	btProcs := make(builtProcessors)
	for i, procId := range processors {
		procCfg, existsCfg := procBuilder.config.Processors[procId]
		if !existsCfg {
			return nil, fmt.Errorf("processor %q is not configured", procId)
		}

		factory, existsFactory := procBuilder.factories[procCfg.ID().Type()]
		if !existsFactory {
			return nil, fmt.Errorf("processor factory for type %q is not configured", procId.Type())
		}

		componentLogger := procBuilder.set.Logger.With(zap.String(zapKindKey, zapKindProcessor), zap.Stringer(zapNameKey, procCfg.ID()))
		creationParams := component.ProcessorCreateSettings{
			Logger:         componentLogger,
			BuildInfo:      procBuilder.set.BuildInfo,
			TracerProvider: procBuilder.set.TracerProvider,
		}

		nextProc := &builtProcessor{}
		mutatesConsumedData := false
		switch dataType {
		case config.TracesDataType:
			var proc component.TracesProcessor
			proc, err = factory.CreateTracesProcessor(ctx, creationParams, procCfg, nextProc)
			if proc != nil {
				mutatesConsumedData = proc.Capabilities().MutatesData
			}
			btProc := &builtProcessor{config.TracesDataType, procCfg, factory, procBuilder.set, proc, nil, nil, nextProc, nil, nil, mutatesConsumedData, proc, procId}
			btProcs[i] = btProc
		case config.MetricsDataType:
			var proc component.MetricsProcessor
			proc, err = factory.CreateMetricsProcessor(ctx, creationParams, procCfg, nextProc)
			if proc != nil {
				mutatesConsumedData = proc.Capabilities().MutatesData
			}
			btProc := &builtProcessor{config.MetricsDataType, procCfg, factory, procBuilder.set, nil, proc, nil, nil, nextProc, nil, mutatesConsumedData, proc, procId}
			btProcs[i] = btProc
		case config.LogsDataType:
			var proc component.LogsProcessor
			proc, err = factory.CreateLogsProcessor(ctx, creationParams, procCfg, nextProc)
			if proc != nil {
				mutatesConsumedData = proc.Capabilities().MutatesData
			}
			btProc := &builtProcessor{config.LogsDataType, procCfg, factory, procBuilder.set, nil, nil, proc, nil, nil, nextProc, mutatesConsumedData, proc, procId}
			btProcs[i] = btProc
		default:
			return nil, fmt.Errorf("error creating processor %q , data type %s is not supported",
				procId, dataType)
		}

		if err != nil {
			return nil, fmt.Errorf("error creating processor %q: %v", procId, err)
		}
	}

	return btProcs, nil
}

func BuildProcessors(
	set component.ProcessorCreateSettings,
	config *config.Config,
	factories map[config.Type]component.ProcessorFactory,
	dataType config.DataType,
	processors []config.ComponentID,
) (builtProcessors, error) {
	procBuilder := &processorsBuilder{set, config, factories}
	return procBuilder.buildProcessors(context.Background(), dataType, processors)
}
