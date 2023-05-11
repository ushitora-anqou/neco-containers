package main

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"
)

// getMySQLCluster gets MySQLCluster object from k8s.
// Note that logging is done in this function.
func getMySQLCluster(ctx context.Context, logger *zap.Logger, name string) (*MySQLCluster, error) {
	stdout, stderr, err := execCmd(ctx, "kubectl", "get", "mysqlcluster", "-n", currentNamespace, name, "-ojson")
	if err != nil {
		logger.Error("could not get MySQLCluster", zap.Error(err), zap.String("stderr", string(stderr)))
		return nil, err
	}
	cluster := &MySQLCluster{}
	err = json.Unmarshal(stdout, cluster)
	if err != nil {
		logger.Error("could not unmarshal MySQLCluster", zap.Error(err))
		return nil, err
	}
	return cluster, nil
}

// getPod gets Pod object from k8s.
// Note that logging is done in this function.
func getPod(ctx context.Context, logger *zap.Logger, name string) (*Pod, error) {
	stdout, stderr, err := execCmd(ctx, "kubectl", "get", "pod", "-n", currentNamespace, name, "-ojson")
	if err != nil {
		logger.Error("could not get Pod", zap.Error(err), zap.String("stderr", string(stderr)))
		return nil, err
	}
	pod := &Pod{}
	err = json.Unmarshal(stdout, pod)
	if err != nil {
		logger.Error("could not unmarshal Pod", zap.Error(err))
		return nil, err
	}
	return pod, nil
}
