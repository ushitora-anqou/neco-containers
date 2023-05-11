package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// rolloutOperation implements "rollout" operation using "kubectl rollout restart".
type rolloutOperation struct {
	clusterName           string
	logger                *zap.Logger
	podCreationTimestamps []string
	previousPrimaryIndex  int
}

func NewRolloutOperation(logger *zap.Logger, clusterName string) Operation {
	return &rolloutOperation{
		clusterName: clusterName,
		logger:      logger.With(zap.String("operation", "rollout")),
	}
}

func (m *rolloutOperation) Name() string {
	return "rollout"
}

func (m *rolloutOperation) ClusterName() string {
	return m.clusterName
}

func (m *rolloutOperation) CheckPreCondition(ctx context.Context) (bool, error) {
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

	m.podCreationTimestamps = make([]string, cluster.Spec.Replicas)
	for i := 0; i < cluster.Spec.Replicas; i++ {
		pod, err := getPod(ctx, m.logger, fmt.Sprintf("moco-%s-%d", m.clusterName, i))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return false, err
			}
			return false, nil
		}
		m.podCreationTimestamps[i] = pod.ObjectMeta.CreationTimestamp
	}

	return true, nil
}

func (m *rolloutOperation) Execute(ctx context.Context) error {
	m.logger.Info("executing rollout")

	_, stderr, err := execCmd(ctx, "kubectl", "rollout", "restart", "-n", currentNamespace, "sts/moco-"+m.clusterName)
	if err != nil {
		m.logger.Error("could not rollout", zap.Error(err), zap.String("stderr", string(stderr)))
		return err
	}

	return nil
}

func (m *rolloutOperation) CheckCompletion(ctx context.Context) (bool, error) {
	cluster, err := getMySQLCluster(ctx, m.logger, m.clusterName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		return false, nil
	}
	if !cluster.Available() {
		return false, nil
	}

	for i := 0; i < cluster.Spec.Replicas; i++ {
		stdout, _, err := execCmd(ctx, "kubectl", "get", "pod", "-n", currentNamespace, fmt.Sprintf("moco-%s-%d", m.clusterName, i), "-ojson")
		if err != nil {
			// not found is not error
			return false, nil
		}
		pod := &Pod{}
		err = json.Unmarshal(stdout, pod)
		if err != nil {
			m.logger.Error("could not unmarshal Pod", zap.Error(err))
			return false, err
		}
		if pod.CreationTimestamp == m.podCreationTimestamps[i] {
			return false, nil
		}
	}

	m.logger.Info("completion confirmed", zap.Int("previous", m.previousPrimaryIndex), zap.Int("current", cluster.Status.CurrentPrimaryIndex))

	return true, nil
}
