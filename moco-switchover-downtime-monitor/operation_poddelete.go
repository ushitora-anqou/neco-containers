package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	"go.uber.org/zap"
)

// poddeleteOperation implements "poddelete" operation using "kubectl delete pod".
type poddeleteOperation struct {
	clusterName                string
	logger                     *zap.Logger
	primary                    bool
	targetIndex                int
	targetPodName              string
	targetPodCreationTimestamp string // not necessarily be time.Time
	previousPrimaryIndex       int
}

func newPoddeleteOperation(logger *zap.Logger, clusterName string, primary bool) Operation {
	m := &poddeleteOperation{
		clusterName: clusterName,
		logger:      logger, // temporary
		primary:     primary,
	}

	m.logger = logger.With(zap.String("operation", m.Name()))

	return m
}

func NewPoddeletePrimaryOperation(logger *zap.Logger, clusterName string) Operation {
	return newPoddeleteOperation(logger, clusterName, true)
}

func NewPoddeleteReplicaOperation(logger *zap.Logger, clusterName string) Operation {
	return newPoddeleteOperation(logger, clusterName, false)
}

func (m *poddeleteOperation) Name() string {
	if m.primary {
		return "poddelete-primary"
	} else {
		return "poddelete-replica"
	}
}

func (m *poddeleteOperation) CheckPreCondition(ctx context.Context) (bool, error) {
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

	if m.primary {
		m.targetIndex = cluster.Status.CurrentPrimaryIndex
	} else {
		m.targetIndex = rand.Intn(cluster.Spec.Replicas - 1)
		if m.targetIndex >= cluster.Status.CurrentPrimaryIndex {
			m.targetIndex++
		}
	}
	m.targetPodName = fmt.Sprintf("moco-%s-%d", m.clusterName, m.targetIndex)

	pod, err := getPod(ctx, m.logger, m.targetPodName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		return false, nil
	}
	m.targetPodCreationTimestamp = pod.CreationTimestamp

	return true, nil
}

func (m *poddeleteOperation) Execute(ctx context.Context) error {
	m.logger.Info("executing poddelete", zap.Int("index", m.targetIndex))

	_, stderr, err := execCmd(ctx, "kubectl", "delete", "pod", "-n", currentNamespace, m.targetPodName, "--wait=false")
	if err != nil {
		m.logger.Error("could not delete pod", zap.Error(err), zap.String("stderr", string(stderr)))
		return err
	}

	return nil
}

func (m *poddeleteOperation) CheckCompletion(ctx context.Context) (bool, error) {
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

	stdout, _, err := execCmd(ctx, "kubectl", "get", "pod", "-n", currentNamespace, m.targetPodName, "-ojson")
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
	if pod.CreationTimestamp == m.targetPodCreationTimestamp {
		return false, nil
	}

	m.logger.Info("completion confirmed", zap.Int("previous", m.previousPrimaryIndex), zap.Int("current", cluster.Status.CurrentPrimaryIndex))

	return true, nil
}
