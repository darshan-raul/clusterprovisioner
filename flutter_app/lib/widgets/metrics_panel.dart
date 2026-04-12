// lib/widgets/metrics_panel.dart

import 'package:flutter/material.dart';
import 'package:fl_chart/fl_chart.dart';
import '../models/cluster_state.dart';

class MetricsPanel extends StatelessWidget {
  final ClusterState state;

  const MetricsPanel({super.key, required this.state});

  @override
  Widget build(BuildContext context) {
    if (state.status != ClusterStatus.running) {
      return const SizedBox.shrink();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Padding(
          padding: EdgeInsets.symmetric(vertical: 8),
          child: Text(
            'LIVE METRICS',
            style: TextStyle(
              color: Color(0xFF8B9BC8),
              fontSize: 12,
              fontWeight: FontWeight.w600,
              letterSpacing: 1.2,
            ),
          ),
        ),
        Row(
          children: [
            Expanded(
              child: _GaugeCard(
                label: 'CPU',
                value: state.cpuUtilization,
                color: const Color(0xFF7C4DFF),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: _GaugeCard(
                label: 'Memory',
                value: state.memoryUtilization,
                color: const Color(0xFF00BCD4),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        _NodeCountCard(nodeCount: state.nodeCount),
      ],
    );
  }
}

class _GaugeCard extends StatelessWidget {
  final String label;
  final double value; // 0–100
  final Color color;

  const _GaugeCard({
    required this.label,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final clamped = value.clamp(0.0, 100.0);
    return Card(
      color: const Color(0xFF1E2433),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      elevation: 4,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            SizedBox(
              height: 120,
              child: Stack(
                alignment: Alignment.center,
                children: [
                  PieChart(
                    PieChartData(
                      startDegreeOffset: -90,
                      sectionsSpace: 0,
                      centerSpaceRadius: 38,
                      sections: [
                        PieChartSectionData(
                          value: clamped,
                          color: color,
                          radius: 12,
                          showTitle: false,
                        ),
                        PieChartSectionData(
                          value: 100 - clamped,
                          color: color.withOpacity(0.1),
                          radius: 12,
                          showTitle: false,
                        ),
                      ],
                    ),
                  ),
                  Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        '${clamped.toStringAsFixed(1)}%',
                        style: TextStyle(
                          color: color,
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: 4),
            Text(
              label,
              style: const TextStyle(
                color: Color(0xFF8B9BC8),
                fontSize: 12,
                fontWeight: FontWeight.w600,
                letterSpacing: 0.8,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _NodeCountCard extends StatelessWidget {
  final int nodeCount;

  const _NodeCountCard({required this.nodeCount});

  @override
  Widget build(BuildContext context) {
    return Card(
      color: const Color(0xFF1E2433),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      elevation: 4,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: const Color(0xFF00E676).withOpacity(0.12),
                borderRadius: BorderRadius.circular(12),
              ),
              child: const Icon(
                Icons.memory_rounded,
                color: Color(0xFF00E676),
                size: 22,
              ),
            ),
            const SizedBox(width: 16),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Active Nodes',
                  style: TextStyle(color: Color(0xFF8B9BC8), fontSize: 12),
                ),
                const SizedBox(height: 2),
                Text(
                  '$nodeCount',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
