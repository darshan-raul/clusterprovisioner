import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/api_service.dart';

class ProvisionScreen extends StatefulWidget {
  const ProvisionScreen({super.key});

  @override
  State<ProvisionScreen> createState() => _ProvisionScreenState();
}

class _ProvisionScreenState extends State<ProvisionScreen> {
  final _nameCtrl = TextEditingController();
  final _awsAccountCtrl = TextEditingController(); // For MVP we ask user here
  String _region = 'ap-south-1';
  String _instanceType = 't3.medium';

  bool _isProvisioning = false;

  void _provision() async {
    if (_nameCtrl.text.isEmpty || _awsAccountCtrl.text.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Fill all fields')));
      return;
    }

    setState(() => _isProvisioning = true);
    try {
      final api = Provider.of<ApiService>(context, listen: false);
      final cluster = await api.provisionCluster(
        name: _nameCtrl.text,
        region: _region,
        instanceType: _instanceType,
        awsAccountId: _awsAccountCtrl.text,
      );
      Navigator.pop(context, cluster.clusterId);
    } catch (e) {
      setState(() => _isProvisioning = false);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Provision Cluster')),
      body: Center(
        child: Container(
          width: 500,
          padding: const EdgeInsets.all(32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('CLUSTER NAME'),
              const SizedBox(height: 8),
              TextField(controller: _nameCtrl, decoration: const InputDecoration(hintText: 'e.g. PROD-NORTH-01')),
              const SizedBox(height: 16),
              
              const Text('AWS ACCOUNT ID (Dest)'),
              const SizedBox(height: 8),
              TextField(controller: _awsAccountCtrl, decoration: const InputDecoration(hintText: '12-digit ID')),
              const SizedBox(height: 16),

              const Text('REGION'),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: _region,
                items: ['ap-south-1', 'us-east-1', 'us-west-2']
                    .map((r) => DropdownMenuItem(value: r, child: Text(r)))
                    .toList(),
                onChanged: (val) => setState(() => _region = val!),
                decoration: const InputDecoration(),
              ),
              const SizedBox(height: 16),

              const Text('INSTANCE TYPE'),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: _instanceType,
                items: ['t3.medium', 't3.large', 'm5.large']
                    .map((r) => DropdownMenuItem(value: r, child: Text(r)))
                    .toList(),
                onChanged: (val) => setState(() => _instanceType = val!),
                decoration: const InputDecoration(),
              ),
              const SizedBox(height: 32),

              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _isProvisioning ? null : _provision,
                  child: _isProvisioning
                     ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
                     : const Text('LAUNCH CLUSTER'),
                ),
              )
            ],
          ),
        ),
      ),
    );
  }
}
