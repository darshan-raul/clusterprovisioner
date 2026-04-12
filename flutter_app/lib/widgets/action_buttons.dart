// lib/widgets/action_buttons.dart

import 'package:flutter/material.dart';
import '../models/cluster_state.dart';

class ActionButtons extends StatelessWidget {
  final ClusterState state;
  final bool isLoading;
  final VoidCallback onStart;
  final VoidCallback onStop;

  const ActionButtons({
    super.key,
    required this.state,
    required this.isLoading,
    required this.onStart,
    required this.onStop,
  });

  @override
  Widget build(BuildContext context) {
    final canStart = state.status == ClusterStatus.stopped && !isLoading;
    final canStop = state.status == ClusterStatus.running && !isLoading;

    return Column(
      children: [
        _ActionButton(
          label: 'Start Cluster',
          icon: Icons.play_arrow_rounded,
          color: const Color(0xFF00E676),
          enabled: canStart,
          isLoading: isLoading && state.status == ClusterStatus.provisioning,
          onPressed: canStart
              ? () => _confirm(
                    context,
                    title: 'Start Cluster?',
                    body:
                        'This will provision a single-node EKS cluster (~8-10 min). '
                        'Auto-teardown fires after 4 hours.',
                    confirmLabel: 'Start',
                    confirmColor: const Color(0xFF00E676),
                    onConfirm: onStart,
                  )
              : null,
        ),
        const SizedBox(height: 12),
        _ActionButton(
          label: 'Stop Cluster',
          icon: Icons.stop_rounded,
          color: const Color(0xFFFF5252),
          enabled: canStop,
          isLoading: isLoading && state.status == ClusterStatus.deprovisioning,
          onPressed: canStop
              ? () => _confirm(
                    context,
                    title: 'Stop Cluster?',
                    body:
                        'This will destroy the EKS cluster and all workloads. '
                        'This action cannot be undone.',
                    confirmLabel: 'Stop',
                    confirmColor: const Color(0xFFFF5252),
                    onConfirm: onStop,
                  )
              : null,
        ),
      ],
    );
  }

  Future<void> _confirm(
    BuildContext context, {
    required String title,
    required String body,
    required String confirmLabel,
    required Color confirmColor,
    required VoidCallback onConfirm,
  }) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1E2433),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(title,
            style: const TextStyle(color: Colors.white, fontSize: 18)),
        content: Text(body,
            style: const TextStyle(color: Color(0xFF8B9BC8), fontSize: 14)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel',
                style: TextStyle(color: Color(0xFF8B9BC8))),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: confirmColor,
              foregroundColor: Colors.black,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8)),
            ),
            onPressed: () => Navigator.pop(context, true),
            child: Text(confirmLabel,
                style: const TextStyle(fontWeight: FontWeight.bold)),
          ),
        ],
      ),
    );
    if (confirmed == true) onConfirm();
  }
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;
  final bool enabled;
  final bool isLoading;
  final VoidCallback? onPressed;

  const _ActionButton({
    required this.label,
    required this.icon,
    required this.color,
    required this.enabled,
    required this.isLoading,
    this.onPressed,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      height: 52,
      child: ElevatedButton.icon(
        style: ElevatedButton.styleFrom(
          backgroundColor: enabled ? color.withOpacity(0.15) : const Color(0xFF1A1F2E),
          foregroundColor: enabled ? color : const Color(0xFF3A4260),
          side: BorderSide(
            color: enabled ? color.withOpacity(0.6) : const Color(0xFF2A3050),
            width: 1.5,
          ),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(14),
          ),
          elevation: 0,
        ),
        onPressed: onPressed,
        icon: isLoading
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: color,
                ),
              )
            : Icon(icon, size: 22),
        label: Text(
          label,
          style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
        ),
      ),
    );
  }
}
