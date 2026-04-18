import 'dart:convert';
import 'package:http/http.dart' as http;
import '../config.dart';
import '../models/cluster.dart';
import 'auth_service.dart';

class ApiService {
  final AuthService _auth;

  ApiService(this._auth);

  Future<Map<String, String>> get _headers async {
    final token = await _auth.getIdToken();
    return {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    };
  }

  Future<List<Cluster>> getClusters() async {
    // For MVP we don't have a GET /clusters array endpoint returning all clusters 
    // unless we modified API Gateway or we just pretend. For today we can just 
    // fetch the single cluster using the UI flow. But wait, we need a dashboard.
    // Let's implement a dummy list that we'll populate from known cluster IDs
    return [];
  }

  Future<Cluster> provisionCluster({
    required String name,
    required String region,
    required String instanceType,
    required String awsAccountId,
  }) async {
    final headers = await _headers;
    final body = jsonEncode({
      'name': name,
      'provider': 'aws',
      'region': region,
      'instance_type': instanceType,
      'aws_account_id': awsAccountId,
    });

    final res = await http.post(
      Uri.parse('$kApiBaseUrl/clusters'),
      headers: headers,
      body: body,
    );

    if (res.statusCode == 202) {
      final json = jsonDecode(res.body);
      return Cluster(
        clusterId: json['cluster_id'],
        status: json['status'],
        name: name,
        provider: 'aws',
        region: region,
        instanceType: instanceType,
        currentStep: 'STARTED',
      );
    } else {
      throw Exception('Failed to provision: \${res.body}');
    }
  }

  Future<Cluster> getCluster(String clusterId) async {
    final headers = await _headers;
    final res = await http.get(
      Uri.parse('$kApiBaseUrl/clusters/\$clusterId'),
      headers: headers,
    );

    if (res.statusCode == 200) {
      return Cluster.fromJson(jsonDecode(res.body));
    } else {
      throw Exception('Failed to get cluster: \${res.body}');
    }
  }
}
