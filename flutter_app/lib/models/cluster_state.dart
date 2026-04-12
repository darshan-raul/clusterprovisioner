// lib/models/cluster_state.dart

enum ClusterStatus {
  running,
  stopped,
  provisioning,
  deprovisioning,
  error,
  unknown,
}

class ClusterState {
  final ClusterStatus status;
  final String? clusterName;
  final String? k8sVersion;
  final int nodeCount;
  final double cpuUtilization;
  final double memoryUtilization;
  final int uptimeMinutes;

  const ClusterState({
    required this.status,
    this.clusterName,
    this.k8sVersion,
    this.nodeCount = 0,
    this.cpuUtilization = 0.0,
    this.memoryUtilization = 0.0,
    this.uptimeMinutes = 0,
  });

  factory ClusterState.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>? ?? {};
    return ClusterState(
      status: _parseStatus(json['status'] as String),
      clusterName: data['cluster_name'] as String?,
      k8sVersion: data['k8s_version'] as String?,
      nodeCount: (data['node_count'] as num?)?.toInt() ?? 0,
      cpuUtilization: (data['cpu_utilization'] as num?)?.toDouble() ?? 0.0,
      memoryUtilization:
          (data['memory_utilization'] as num?)?.toDouble() ?? 0.0,
      uptimeMinutes: (data['uptime_minutes'] as num?)?.toInt() ?? 0,
    );
  }

  static ClusterStatus _parseStatus(String s) => switch (s) {
        'RUNNING' => ClusterStatus.running,
        'STOPPED' => ClusterStatus.stopped,
        'PROVISIONING' => ClusterStatus.provisioning,
        'DEPROVISIONING' => ClusterStatus.deprovisioning,
        'ERROR' => ClusterStatus.error,
        _ => ClusterStatus.unknown,
      };

  /// Human-readable label for the status
  String get statusLabel => switch (status) {
        ClusterStatus.running => 'RUNNING',
        ClusterStatus.stopped => 'STOPPED',
        ClusterStatus.provisioning => 'PROVISIONING',
        ClusterStatus.deprovisioning => 'DEPROVISIONING',
        ClusterStatus.error => 'ERROR',
        ClusterStatus.unknown => 'UNKNOWN',
      };

  bool get isTransitioning =>
      status == ClusterStatus.provisioning ||
      status == ClusterStatus.deprovisioning;
}
