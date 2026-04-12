// lib/main.dart

import 'package:flutter/material.dart';
import 'screens/dashboard_screen.dart';

void main() {
  runApp(const EksControlApp());
}

class EksControlApp extends StatelessWidget {
  const EksControlApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'EKS Control Panel',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.dark(
          primary: const Color(0xFF7C4DFF),
          secondary: const Color(0xFF00E676),
          surface: const Color(0xFF1E2433),
          error: const Color(0xFFEF5350),
        ),
        scaffoldBackgroundColor: const Color(0xFF0D1117),
        fontFamily: 'Roboto',
        cardTheme: const CardTheme(
          color: Color(0xFF1E2433),
          elevation: 4,
        ),
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF0D1117),
          elevation: 0,
          titleTextStyle: TextStyle(
            color: Colors.white,
            fontSize: 18,
            fontWeight: FontWeight.w700,
          ),
        ),
        useMaterial3: true,
      ),
      home: const DashboardScreen(),
    );
  }
}
