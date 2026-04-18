class Cluster {
  final String clusterId;
  final String name;
  final String provider;
  final String region;
  final String instanceType;
  final String status;
  final String currentStep;
  final String? clusterEndpoint;
  final String? error;

  Cluster({
    required this.clusterId,
    required this.name,
    required this.provider,
    required this.region,
    required this.instanceType,
    required this.status,
    required this.currentStep,
    this.clusterEndpoint,
    this.error,
  });

  factory Cluster.fromJson(Map<String, dynamic> json) {
    return Cluster(
      clusterId: json['cluster_id'] ?? '',
      name: json['name'] ?? '',
      provider: json['provider'] ?? '',
      region: json['region'] ?? '',
      instanceType: json['instance_type'] ?? '',
      status: json['status'] ?? 'UNKNOWN',
      currentStep: json['current_step'] ?? '',
      clusterEndpoint: json['cluster_endpoint'],
      error: json['error_message'],
    );
  }
}
