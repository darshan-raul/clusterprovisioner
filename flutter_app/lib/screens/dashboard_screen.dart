// lib/screens/dashboard_screen.dart

import 'dart:async';
import 'package:flutter/material.dart';
import '../models/cluster_state.dart';
import '../services/api_service.dart';
import '../widgets/cluster_status_card.dart';
import '../widgets/metrics_panel.dart';
import '../widgets/action_buttons.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  final _api = ApiService();

  ClusterState _state = const ClusterState(status: ClusterStatus.unknown);
  bool _isLoading = false;
  bool _isFetching = false;
  String? _errorMessage;

  Timer? _pollTimer;
  static const _pollInterval = Duration(seconds: 10);

  @override
  void initState() {
    super.initState();
    _fetchStatus();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  // ─── Polling ─────────────────────────────────────────────────────────────

  void _startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(_pollInterval, (_) => _fetchStatus());
  }

  void _stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  // ─── API calls ───────────────────────────────────────────────────────────

  Future<void> _fetchStatus() async {
    if (_isFetching) return;
    setState(() => _isFetching = true);
    try {
      final json = await _api.getStatus();
      final newState = ClusterState.fromJson(json);
      setState(() {
        _state = newState;
        _errorMessage = null;
      });

      // Auto-manage polling based on transitional state
      if (newState.isTransitioning) {
        _startPolling();
      } else {
        _stopPolling();
      }
    } catch (e) {
      setState(() => _errorMessage = e.toString());
    } finally {
      setState(() => _isFetching = false);
    }
  }

  Future<void> _startCluster() async {
    setState(() {
      _isLoading = true;
      // Optimistic update
      _state = ClusterState(
        status: ClusterStatus.provisioning,
        clusterName: _state.clusterName,
      );
    });

    try {
      final json = await _api.startCluster();
      final msg = json['message'] as String? ?? 'Cluster provisioning started';
      if (mounted) _showSnackBar(msg, isError: false);
      _startPolling();
    } catch (e) {
      if (mounted) _showSnackBar('Failed: $e', isError: true);
      await _fetchStatus(); // Roll back optimistic state
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  Future<void> _stopCluster() async {
    setState(() {
      _isLoading = true;
      // Optimistic update
      _state = ClusterState(
        status: ClusterStatus.deprovisioning,
        clusterName: _state.clusterName,
      );
    });

    try {
      final json = await _api.stopCluster();
      final msg = json['message'] as String? ?? 'Cluster teardown started';
      if (mounted) _showSnackBar(msg, isError: false);
      _startPolling();
    } catch (e) {
      if (mounted) _showSnackBar('Failed: $e', isError: true);
      await _fetchStatus();
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  // ─── UI helpers ──────────────────────────────────────────────────────────

  void _showSnackBar(String message, {required bool isError}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor:
            isError ? const Color(0xFFEF5350) : const Color(0xFF1E2433),
        behavior: SnackBarBehavior.floating,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        margin: const EdgeInsets.all(12),
      ),
    );
  }

  // ─── Build ───────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0D1117),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D1117),
        elevation: 0,
        title: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(6),
              decoration: BoxDecoration(
                color: const Color(0xFF7C4DFF).withOpacity(0.15),
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Icon(
                Icons.cloud_rounded,
                color: Color(0xFF7C4DFF),
                size: 20,
              ),
            ),
            const SizedBox(width: 10),
            const Text(
              'EKS Control Panel',
              style: TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
        actions: [
          if (_isFetching)
            const Padding(
              padding: EdgeInsets.only(right: 16),
              child: Center(
                child: SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Color(0xFF7C4DFF),
                  ),
                ),
              ),
            )
          else
            IconButton(
              tooltip: 'Refresh',
              icon: const Icon(Icons.refresh_rounded, color: Color(0xFF8B9BC8)),
              onPressed: _fetchStatus,
            ),
        ],
      ),
      body: RefreshIndicator(
        color: const Color(0xFF7C4DFF),
        backgroundColor: const Color(0xFF1E2433),
        onRefresh: _fetchStatus,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
          children: [
            if (_errorMessage != null) _ErrorBanner(message: _errorMessage!),
            const SizedBox(height: 8),
            ClusterStatusCard(state: _state),
            const SizedBox(height: 16),
            MetricsPanel(state: _state),
            if (_state.status == ClusterStatus.running)
              const SizedBox(height: 16),
            const SizedBox(height: 8),
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 8),
              child: Text(
                'ACTIONS',
                style: TextStyle(
                  color: Color(0xFF8B9BC8),
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  letterSpacing: 1.2,
                ),
              ),
            ),
            ActionButtons(
              state: _state,
              isLoading: _isLoading,
              onStart: _startCluster,
              onStop: _stopCluster,
            ),
            const SizedBox(height: 24),
            _AutoTeardownBanner(state: _state),
          ],
        ),
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String message;
  const _ErrorBanner({required this.message});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      margin: const EdgeInsets.only(bottom: 8),
      decoration: BoxDecoration(
        color: const Color(0xFFEF5350).withOpacity(0.12),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFEF5350).withOpacity(0.4)),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline_rounded,
              color: Color(0xFFEF5350), size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: const TextStyle(color: Color(0xFFEF5350), fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }
}

class _AutoTeardownBanner extends StatelessWidget {
  final ClusterState state;
  const _AutoTeardownBanner({required this.state});

  @override
  Widget build(BuildContext context) {
    if (state.status != ClusterStatus.running) return const SizedBox.shrink();
    final remaining = 240 - state.uptimeMinutes; // 4h = 240 min
    if (remaining <= 0) return const SizedBox.shrink();

    final h = remaining ~/ 60;
    final m = remaining % 60;
    final label = h > 0 ? '${h}h ${m}m' : '${m}m';

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFB300).withOpacity(0.1),
        borderRadius: BorderRadius.circular(12),
        border:
            Border.all(color: const Color(0xFFFFB300).withOpacity(0.3)),
      ),
      child: Row(
        children: [
          const Icon(Icons.alarm_rounded,
              color: Color(0xFFFFB300), size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Auto-teardown in $label',
              style: const TextStyle(
                  color: Color(0xFFFFB300), fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }
}
