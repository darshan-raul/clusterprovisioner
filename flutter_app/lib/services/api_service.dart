// lib/services/api_service.dart

import 'dart:convert';
import 'package:http/http.dart' as http;

class ApiService {
  // TODO: Replace these with your actual values after deploying infra/
  static const _baseUrl =
      'https://<YOUR_API_ID>.execute-api.ap-south-1.amazonaws.com/prod';
  static const _apiKey = '<YOUR_API_KEY>';

  static Map<String, String> get _headers => {
        'x-api-key': _apiKey,
        'Content-Type': 'application/json',
      };

  Future<Map<String, dynamic>> getStatus() async {
    final res = await http.get(
      Uri.parse('$_baseUrl/cluster/status'),
      headers: _headers,
    );
    _checkStatus(res);
    return json.decode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> startCluster() async {
    final res = await http.post(
      Uri.parse('$_baseUrl/cluster/start'),
      headers: _headers,
    );
    _checkStatus(res);
    return json.decode(res.body) as Map<String, dynamic>;
  }

  Future<Map<String, dynamic>> stopCluster() async {
    final res = await http.post(
      Uri.parse('$_baseUrl/cluster/stop'),
      headers: _headers,
    );
    _checkStatus(res);
    return json.decode(res.body) as Map<String, dynamic>;
  }

  void _checkStatus(http.Response res) {
    if (res.statusCode >= 400) {
      throw Exception(
          'API error ${res.statusCode}: ${res.body}');
    }
  }
}
