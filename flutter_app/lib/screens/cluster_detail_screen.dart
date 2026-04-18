import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/api_service.dart';
import '../models/cluster.dart';
import '../theme/app_theme.dart';

class ClusterDetailScreen extends StatefulWidget {
  final String clusterId;
  ClusterDetailScreen({required this.clusterId});

  @override
  _ClusterDetailScreenState createState() => _ClusterDetailScreenState();
}

class _ClusterDetailScreenState extends State<ClusterDetailScreen> {
  Cluster? _cluster;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _fetch();
    _timer = Timer.periodic(const Duration(seconds: 10), (_) => _fetch());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  void _fetch() async {
    try {
      final api = Provider.of<ApiService>(context, listen: false);
      final c = await api.getCluster(widget.clusterId);
      if (mounted) setState(() => _cluster = c);
      if (c.status == 'READY' || c.status == 'FAILED') {
        _timer?.cancel();
      }
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    if (_cluster == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Cluster Details')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    final c = _cluster!;
    return Scaffold(
      appBar: AppBar(title: Text(c.name)),
      body: Center(
        child: Container(
          width: 600,
          padding: const EdgeInsets.all(32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Text('STATUS: \${c.status}', style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold)),
                  const Spacer(),
                  if (c.status != 'READY' && c.status != 'FAILED')
                    const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)),
                ],
              ),
              const SizedBox(height: 16),
              Text('Current Step: \${c.currentStep}', style: const TextStyle(color: AppTheme.primary)),
              const Divider(height: 48),
              
              const Text('Details', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
              const SizedBox(height: 16),
              Text('Cluster ID: \${c.clusterId}'),
              Text('Region: \${c.region}'),
              Text('Instance Type: \${c.instanceType}'),
              
              if (c.clusterEndpoint != null) ...[
                const SizedBox(height: 24),
                const Text('Cluster Endpoint:', style: TextStyle(fontWeight: FontWeight.bold)),
                SelectableText(c.clusterEndpoint!),
              ],
              
              if (c.error != null) ...[
                const SizedBox(height: 24),
                const Text('Error:', style: TextStyle(fontWeight: FontWeight.bold, color: AppTheme.danger)),
                SelectableText(c.error!, style: const TextStyle(color: AppTheme.danger)),
              ]
            ],
          ),
        ),
      ),
    );
  }
}
