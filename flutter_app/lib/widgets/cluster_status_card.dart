// lib/widgets/cluster_status_card.dart

import 'package:flutter/material.dart';
import '../models/cluster_state.dart';

class ClusterStatusCard extends StatelessWidget {
  final ClusterState state;

  const ClusterStatusCard({super.key, required this.state});

  @override
  Widget build(BuildContext context) {
    return Card(
      color: const Color(0xFF1E2433),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      elevation: 4,
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text(
                  'Cluster Status',
                  style: TextStyle(
                    color: Color(0xFF8B9BC8),
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    letterSpacing: 1.2,
                  ),
                ),
                _StatusBadge(status: state.status),
              ],
            ),
            const SizedBox(height: 16),
            _InfoRow(
              icon: Icons.dns_rounded,
              label: 'Cluster',
              value: state.clusterName ?? '—',
            ),
            const SizedBox(height: 8),
            _InfoRow(
              icon: Icons.commit_rounded,
              label: 'K8s Version',
              value: state.k8sVersion != null ? 'v${state.k8sVersion}' : '—',
            ),
            const SizedBox(height: 8),
            _InfoRow(
              icon: Icons.timer_rounded,
              label: 'Uptime',
              value: _formatUptime(state.uptimeMinutes),
            ),
          ],
        ),
      ),
    );
  }

  String _formatUptime(int minutes) {
    if (minutes == 0) return '—';
    final h = minutes ~/ 60;
    final m = minutes % 60;
    if (h > 0) return '${h}h ${m}m';
    return '${m}m';
  }
}

class _StatusBadge extends StatefulWidget {
  final ClusterStatus status;
  const _StatusBadge({required this.status});

  @override
  State<_StatusBadge> createState() => _StatusBadgeState();
}

class _StatusBadgeState extends State<_StatusBadge>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;
  late Animation<double> _pulse;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 800),
    )..repeat(reverse: true);
    _pulse = Tween<double>(begin: 0.5, end: 1.0).animate(_controller);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final color = _statusColor(widget.status);
    final label = _statusLabel(widget.status);
    final isAnimated = widget.status == ClusterStatus.provisioning ||
        widget.status == ClusterStatus.deprovisioning;

    return AnimatedBuilder(
      animation: _pulse,
      builder: (_, __) {
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          decoration: BoxDecoration(
            color: color.withOpacity(isAnimated ? _pulse.value * 0.25 : 0.15),
            borderRadius: BorderRadius.circular(20),
            border: Border.all(
              color: color.withOpacity(isAnimated ? _pulse.value : 0.8),
              width: 1.5,
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  color: color,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 6),
              Text(
                label,
                style: TextStyle(
                  color: color,
                  fontSize: 11,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 0.8,
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Color _statusColor(ClusterStatus s) => switch (s) {
        ClusterStatus.running => const Color(0xFF00E676),
        ClusterStatus.stopped => const Color(0xFF90A4AE),
        ClusterStatus.provisioning => const Color(0xFFFFB300),
        ClusterStatus.deprovisioning => const Color(0xFFFF7043),
        ClusterStatus.error => const Color(0xFFEF5350),
        ClusterStatus.unknown => const Color(0xFF90A4AE),
      };

  String _statusLabel(ClusterStatus s) => switch (s) {
        ClusterStatus.running => 'RUNNING',
        ClusterStatus.stopped => 'STOPPED',
        ClusterStatus.provisioning => 'PROVISIONING',
        ClusterStatus.deprovisioning => 'DEPROVISIONING',
        ClusterStatus.error => 'ERROR',
        ClusterStatus.unknown => 'UNKNOWN',
      };
}

class _InfoRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;

  const _InfoRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 16, color: const Color(0xFF5C73A8)),
        const SizedBox(width: 8),
        Text(
          '$label:',
          style: const TextStyle(
            color: Color(0xFF8B9BC8),
            fontSize: 13,
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            value,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}
