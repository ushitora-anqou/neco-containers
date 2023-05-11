package main

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

// switchoverOperation implements "switchover" operation using "kubectl moco switchover".
type switchoverOperation struct {
	clusterName          string
	logger               *zap.Logger
	previousPrimaryIndex int
}

func NewSwitchoverOperation(logger *zap.Logger, clusterName string) Operation {
	return &switchoverOperation{
		clusterName: clusterName,
		logger:      logger.With(zap.String("operation", "switchover")),
	}
}

func (m *switchoverOperation) Name() string {
	return "switchover"
}

func (m *switchoverOperation) ClusterName() string {
	return m.clusterName
}

func (m *switchoverOperation) CheckPreCondition(ctx context.Context) (bool, error) {
	cluster, err := getMySQLCluster(ctx, m.logger, m.clusterName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		return false, nil
	}

	if !cluster.Healthy() {
		return false, nil
	}

	m.previousPrimaryIndex = cluster.Status.CurrentPrimaryIndex

	return true, nil
}

func (m *switchoverOperation) Execute(ctx context.Context) error {
	m.logger.Info("executing switchover")

	_, stderr, err := execCmd(ctx, "kubectl", "moco", "switchover", "-n", currentNamespace, m.clusterName)
	if err != nil {
		m.logger.Error("could not switchover", zap.Error(err), zap.String("stderr", string(stderr)))
		return err
	}

	return nil
}

func (m *switchoverOperation) CheckCompletion(ctx context.Context) (bool, error) {
	cluster, err := getMySQLCluster(ctx, m.logger, m.clusterName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		return false, nil
	}

	if !cluster.Available() || cluster.Status.CurrentPrimaryIndex == m.previousPrimaryIndex {
		return false, nil
	}

	m.logger.Info("completion confirmed", zap.Int("previous", m.previousPrimaryIndex), zap.Int("current", cluster.Status.CurrentPrimaryIndex))

	return true, nil
}
