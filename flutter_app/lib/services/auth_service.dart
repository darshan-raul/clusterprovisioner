import 'package:amazon_cognito_identity_dart_2/cognito.dart';
import '../config.dart';

class AuthService {
  late final CognitoUserPool _userPool;

  AuthService() {
    _userPool = CognitoUserPool(
      kCognitoUserPoolId.isEmpty ? 'ap-south-1_dummyID' : kCognitoUserPoolId,
      kCognitoAppClientId.isEmpty ? 'dummyClientId' : kCognitoAppClientId,
    );
  }

  CognitoUser? _cognitoUser;
  CognitoUserSession? _session;

  Future<bool> login(String email, String password) async {
    final cognitoUser = CognitoUser(email, _userPool);
    final authDetails = AuthenticationDetails(
      username: email,
      password: password,
    );
    try {
      _session = await cognitoUser.authenticateUser(authDetails);
      _cognitoUser = cognitoUser;
      return true;
    } on CognitoUserException catch (e) {
      throw Exception(e.message ?? 'Login failed');
    } catch (e) {
      throw Exception(e.toString());
    }
  }

  /// Returns the email address so the verify screen knows where to confirm
  Future<String> signUp(String email, String password) async {
    final userAttributes = [
      AttributeArg(name: 'email', value: email),
    ];
    try {
      await _userPool.signUp(email, password, userAttributes: userAttributes);
      return email;
    } on CognitoUserException catch (e) {
      throw Exception(e.message ?? 'Sign up failed');
    } catch (e) {
      throw Exception(e.toString());
    }
  }

  Future<void> confirmSignUp(String email, String code) async {
    final cognitoUser = CognitoUser(email, _userPool);
    try {
      await cognitoUser.confirmRegistration(code);
    } on CognitoUserException catch (e) {
      throw Exception(e.message ?? 'Verification failed');
    } catch (e) {
      throw Exception(e.toString());
    }
  }

  Future<void> resendConfirmationCode(String email) async {
    final cognitoUser = CognitoUser(email, _userPool);
    await cognitoUser.resendConfirmationCode();
  }

  Future<String?> getIdToken() async {
    if (_session != null && _session!.isValid()) {
      return _session!.getIdToken().getJwtToken();
    }
    // refresh token logic could go here
    return null;
  }

  Future<void> logout() async {
    if (_cognitoUser != null) {
      await _cognitoUser!.signOut();
      _cognitoUser = null;
      _session = null;
    }
  }
}
