import 'dart:async';
import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:provider/provider.dart';
import '../services/api_service.dart';
import '../services/auth_service.dart';
import '../models/cluster.dart';
import '../theme/app_theme.dart';
import 'provision_screen.dart';
import 'cluster_detail_screen.dart';
import 'login_screen.dart';

// MVP: track provisioned cluster IDs in-memory
final List<String> myClusterIds = [];

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  bool _isLoading = false;
  List<Cluster> _clusters = [];
  List<Cluster> _filtered = [];
  final _searchCtrl = TextEditingController();
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
    _timer = Timer.periodic(const Duration(seconds: 15), (_) => _load());
    _searchCtrl.addListener(_filter);
  }

  @override
  void dispose() {
    _timer?.cancel();
    _searchCtrl.dispose();
    super.dispose();
  }

  void _filter() {
    final q = _searchCtrl.text.toLowerCase();
    setState(() {
      _filtered = q.isEmpty
          ? _clusters
          : _clusters.where((c) =>
              c.name.toLowerCase().contains(q) ||
              c.region.toLowerCase().contains(q) ||
              c.status.toLowerCase().contains(q)).toList();
    });
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() => _isLoading = true);
    final api = Provider.of<ApiService>(context, listen: false);
    final loaded = <Cluster>[];
    for (final id in myClusterIds) {
      try {
        loaded.add(await api.getCluster(id));
      } catch (_) {}
    }
    if (mounted) {
      setState(() {
        _clusters = loaded;
        _filtered = loaded;
        _isLoading = false;
      });
      _filter();
    }
  }

  int get _totalNodes => _clusters.fold(0, (sum, _) => sum + 2); // 2 nodes per cluster MVP approx

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppTheme.background,
      body: Column(
        children: [
          // Top bar
          _TopBar(onLogout: _logout),

          // Scrollable content
          Expanded(
            child: RefreshIndicator(
              onRefresh: _load,
              color: AppTheme.primary,
              child: ListView(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 120),
                children: [
                  const SizedBox(height: 28),
                  // Header
                  Text(
                    'ACTIVE CLUSTERS',
                    style: GoogleFonts.inter(
                      fontSize: 28,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      letterSpacing: 1.5,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    _clusters.isEmpty
                        ? 'No clusters provisioned yet'
                        : '$_totalNodes nodes across multi-cloud infrastructure',
                    style: GoogleFonts.inter(
                      fontSize: 14,
                      color: Colors.white38,
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Search bar
                  Container(
                    decoration: BoxDecoration(
                      color: const Color(0xFF111827),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: Colors.white.withOpacity(0.07)),
                    ),
                    child: TextField(
                      controller: _searchCtrl,
                      style: const TextStyle(color: Colors.white),
                      decoration: InputDecoration(
                        hintText: 'Search resources...',
                        hintStyle: const TextStyle(color: Colors.white30),
                        prefixIcon: const Icon(Icons.search, color: Colors.white30, size: 20),
                        suffixIcon: const Icon(Icons.tune_rounded, color: Colors.white30, size: 20),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
                      ),
                    ),
                  ),
                  const SizedBox(height: 28),

                  // Cluster list
                  if (_isLoading && _clusters.isEmpty)
                    const Center(
                      child: Padding(
                        padding: EdgeInsets.only(top: 80),
                        child: CircularProgressIndicator(color: AppTheme.primary),
                      ),
                    )
                  else if (_filtered.isEmpty)
                    _EmptyState(onProvision: _goProvision)
                  else
                    ..._filtered.map((c) => Padding(
                          padding: const EdgeInsets.only(bottom: 16),
                          child: _ClusterCard(
                            cluster: c,
                            onTap: () => Navigator.push(
                              context,
                              MaterialPageRoute(builder: (_) => ClusterDetailScreen(clusterId: c.clusterId)),
                            ),
                          ),
                        )),
                ],
              ),
            ),
          ),
        ],
      ),

      // PROVISION CLUSTER button
      floatingActionButton: _ProvisionButton(onTap: _goProvision),
      floatingActionButtonLocation: FloatingActionButtonLocation.centerFloat,
    );
  }

  void _goProvision() async {
    final newId = await Navigator.push<String>(
      context,
      MaterialPageRoute(builder: (_) => const ProvisionScreen()),
    );
    if (newId != null) {
      myClusterIds.add(newId);
      _load();
    }
  }

  void _logout() async {
    await Provider.of<AuthService>(context, listen: false).logout();
    if (mounted) {
      Navigator.pushAndRemoveUntil(
        context,
        MaterialPageRoute(builder: (_) => const LoginScreen()),
        (_) => false,
      );
    }
  }
}

// ─── Top Bar ────────────────────────────────────────────────────────────────

class _TopBar extends StatelessWidget {
  final VoidCallback onLogout;
  const _TopBar({required this.onLogout});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
      decoration: BoxDecoration(
        color: AppTheme.background,
        border: Border(bottom: BorderSide(color: Colors.white.withOpacity(0.06))),
      ),
      child: Row(
        children: [
          Container(
            width: 30,
            height: 30,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(6),
              gradient: const LinearGradient(
                colors: [Color(0xFF60A5FA), Color(0xFF818CF8)],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
            ),
            child: const Icon(Icons.hub_rounded, color: Colors.white, size: 16),
          ),
          const SizedBox(width: 10),
          Text(
            'OBSERVATORY',
            style: GoogleFonts.inter(
              fontSize: 15,
              fontWeight: FontWeight.w700,
              letterSpacing: 2.5,
              color: Colors.white,
            ),
          ),
          const Spacer(),
          GestureDetector(
            onTap: onLogout,
            child: Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(10),
                color: const Color(0xFF1A2235),
                border: Border.all(color: Colors.white.withOpacity(0.08)),
              ),
              child: const Icon(Icons.person_outline_rounded, color: Colors.white54, size: 18),
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Cluster Card ───────────────────────────────────────────────────────────

class _ClusterCard extends StatelessWidget {
  final Cluster cluster;
  final VoidCallback onTap;
  const _ClusterCard({required this.cluster, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final status = cluster.status;
    final isFailed = status == 'FAILED';
    final isReady = status == 'READY';
    final isProvisioning = status == 'PROVISIONING' || status == 'INITIATED';

    final statusColor = isFailed
        ? AppTheme.danger
        : isReady
            ? AppTheme.success
            : AppTheme.warning;

    final statusLabel = isFailed
        ? 'CRITICAL'
        : isReady
            ? 'OPERATIONAL'
            : 'RECONCILING...';

    return GestureDetector(
      onTap: onTap,
      child: Container(
        decoration: BoxDecoration(
          color: const Color(0xFF111827),
          borderRadius: BorderRadius.circular(16),
          border: Border.all(
            color: isFailed
                ? AppTheme.danger.withOpacity(0.4)
                : Colors.white.withOpacity(0.07),
          ),
        ),
        child: Column(
          children: [
            // Card header
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  // Provider icon
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(10),
                      color: isFailed
                          ? AppTheme.danger.withOpacity(0.12)
                          : isReady
                              ? AppTheme.primary.withOpacity(0.12)
                              : AppTheme.success.withOpacity(0.12),
                    ),
                    child: Icon(
                      isFailed
                          ? Icons.warning_amber_rounded
                          : isReady
                              ? Icons.cloud_done_rounded
                              : Icons.cloud_sync_rounded,
                      color: isFailed
                          ? AppTheme.danger
                          : isReady
                              ? AppTheme.primary
                              : AppTheme.success,
                      size: 22,
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          cluster.name.toUpperCase(),
                          style: GoogleFonts.inter(
                            fontSize: 15,
                            fontWeight: FontWeight.w700,
                            color: Colors.white,
                            letterSpacing: 0.5,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Row(
                          children: [
                            _Badge(label: 'AWS', color: AppTheme.primary),
                            const SizedBox(width: 8),
                            Text(
                              cluster.region.toUpperCase(),
                              style: GoogleFonts.inter(fontSize: 11, color: Colors.white38),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                  Icon(Icons.delete_outline_rounded, color: Colors.white24, size: 20),
                ],
              ),
            ),

            // Divider
            Divider(height: 1, color: Colors.white.withOpacity(0.06)),

            // Stats row
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: _StatBox(
                      label: isProvisioning ? 'SYNC PROGRESS' : 'STATUS',
                      child: Row(
                        children: [
                          Container(
                            width: 7,
                            height: 7,
                            decoration: BoxDecoration(
                              color: isProvisioning ? Colors.grey : statusColor,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 6),
                          Text(
                            statusLabel,
                            style: GoogleFonts.inter(
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                              color: isProvisioning ? Colors.white54 : statusColor,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _StatBox(
                      label: isReady ? 'INSTANCE' : 'STEP',
                      child: Text(
                        isReady
                            ? cluster.instanceType
                            : cluster.currentStep.replaceAll('_', ' '),
                        style: GoogleFonts.inter(
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                          color: AppTheme.primary,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // Error banner
            if (isFailed && cluster.error != null)
              Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                decoration: BoxDecoration(
                  color: AppTheme.danger.withOpacity(0.12),
                  borderRadius: const BorderRadius.only(
                    bottomLeft: Radius.circular(16),
                    bottomRight: Radius.circular(16),
                  ),
                ),
                child: Text(
                  cluster.error!,
                  style: GoogleFonts.inter(
                    fontSize: 12,
                    color: AppTheme.danger,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

// ─── Badge ───────────────────────────────────────────────────────────────────

class _Badge extends StatelessWidget {
  final String label;
  final Color color;
  const _Badge({required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withOpacity(0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: GoogleFonts.inter(
          fontSize: 10,
          fontWeight: FontWeight.w700,
          color: color,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}

// ─── Stat Box ────────────────────────────────────────────────────────────────

class _StatBox extends StatelessWidget {
  final String label;
  final Widget child;
  const _StatBox({required this.label, required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF0D1422),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: GoogleFonts.inter(
              fontSize: 10,
              letterSpacing: 1,
              color: Colors.white30,
            ),
          ),
          const SizedBox(height: 6),
          child,
        ],
      ),
    );
  }
}

// ─── Empty State ─────────────────────────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  final VoidCallback onProvision;
  const _EmptyState({required this.onProvision});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.only(top: 80),
        child: Column(
          children: [
            Container(
              width: 72,
              height: 72,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: AppTheme.primary.withOpacity(0.08),
                border: Border.all(color: AppTheme.primary.withOpacity(0.2)),
              ),
              child: const Icon(Icons.cloud_off_rounded, color: AppTheme.primary, size: 32),
            ),
            const SizedBox(height: 20),
            Text(
              'No clusters yet',
              style: GoogleFonts.inter(fontSize: 18, fontWeight: FontWeight.w700, color: Colors.white),
            ),
            const SizedBox(height: 8),
            Text(
              'Provision your first EKS cluster\nto get started.',
              style: GoogleFonts.inter(fontSize: 14, color: Colors.white38, height: 1.6),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Provision FAB ──────────────────────────────────────────────────────────

class _ProvisionButton extends StatelessWidget {
  final VoidCallback onTap;
  const _ProvisionButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: SizedBox(
        width: double.infinity,
        child: ElevatedButton.icon(
          onPressed: onTap,
          icon: const Icon(Icons.add, size: 18),
          label: Text(
            'PROVISION CLUSTER',
            style: GoogleFonts.inter(fontSize: 14, fontWeight: FontWeight.w700, letterSpacing: 1.5),
          ),
          style: ElevatedButton.styleFrom(
            backgroundColor: AppTheme.primary,
            foregroundColor: const Color(0xFF0A0E1A),
            padding: const EdgeInsets.symmetric(vertical: 18),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
            elevation: 0,
          ),
        ),
      ),
    );
  }
}
