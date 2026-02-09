## Summary

This PR includes comprehensive refactoring focused on security, design improvements, and code quality enhancements for the v1.0.2 release.

## Security Improvements

- **File Permissions**: Fixed file permissions to be more restrictive (0700 for directories, 0600 for files containing sensitive data like API keys)
- **Input Validation**: Added comprehensive input validation throughout the codebase
- **Path Traversal Prevention**: Added hash validation and path traversal checks in cache operations
- **Error Handling**: Improved error handling to prevent information leakage

## Design Improvements

- **Constants**: Replaced magic numbers with named constants (ConfigDirPerm, ConfigFilePerm, CacheDirPerm, CacheFilePerm)
- **Error Handling**: Consistent error wrapping using fmt.Errorf() with %w verb
- **Context Handling**: Added proper context cancellation checks in streaming functions
- **Resource Cleanup**: Improved resource cleanup with proper defer usage

## Code Quality

- **Validation**: Added validation for API keys, models, modes, and other inputs
- **Error Messages**: More descriptive and consistent error messages
- **Tests**: Fixed cache tests to use valid SHA256 hashes

## Testing

All existing tests pass. No functional changes - the app works exactly as before.

## Version

Bumped version to v1.0.2
