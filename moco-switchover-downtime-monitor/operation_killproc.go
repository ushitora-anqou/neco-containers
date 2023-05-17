package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"

	"go.uber.org/zap"
)

// killprocOperation implements "killproc" operation using "kubectl exec -- kill".
type killprocOperation struct {
	clusterName                 string
	logger                      *zap.Logger
	primary                     bool
	targetIndex                 int
	targetPodName               string
	targetPodMysqldRestartCount int
	previousPrimaryIndex        int
}

func newKillprocOperation(logger *zap.Logger, clusterName string, primary bool) Operation {
	m := &killprocOperation{
		clusterName: clusterName,
		logger:      logger, // temporary
		primary:     primary,
	}

	m.logger = logger.With(zap.String("operation", m.Name()))

	return m
}

func NewKillprocPrimaryOperation(logger *zap.Logger, clusterName string) Operation {
	return newKillprocOperation(logger, clusterName, true)
}

func NewKillprocReplicaOperation(logger *zap.Logger, clusterName string) Operation {
	return newKillprocOperation(logger, clusterName, false)
}

func (m *killprocOperation) Name() string {
	if m.primary {
		return "killproc-primary"
	} else {
		return "killproc-replica"
	}
}

func (m *killprocOperation) CheckPreCondition(ctx context.Context) (bool, error) {
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
	m.targetPodMysqldRestartCount = pod.RestartCount("mysqld")

	return true, nil
}

func (m *killprocOperation) Execute(ctx context.Context) error {
	m.logger.Info("executing killproc", zap.Int("index", m.targetIndex))

	_, stderr, err := execCmdWithInput(ctx,
		[]byte(`exec kill -SEGV $(ps axo pid,command | awk '$2=="mysqld" {print $1}')`),
		"kubectl", "exec", "-n", currentNamespace, m.targetPodName, "-c", "mysqld", "-i", "--", "bash", "/dev/stdin")
	if err != nil {
		m.logger.Error("could not kill process", zap.Error(err), zap.String("stderr", string(stderr)))
		return err
	}

	return nil
}

func (m *killprocOperation) CheckCompletion(ctx context.Context) (bool, error) {
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
	if pod.RestartCount("mysqld") != m.targetPodMysqldRestartCount+1 {
		return false, nil
	}

	m.logger.Info("completion confirmed", zap.Int("previous", m.previousPrimaryIndex), zap.Int("current", cluster.Status.CurrentPrimaryIndex))

	return true, nil
}
