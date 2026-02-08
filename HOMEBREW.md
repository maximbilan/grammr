# Homebrew Installation Setup

This repository includes a Homebrew formula that allows installation directly from this repo.

## How It Works

The `Formula/grammr.rb` file is a Homebrew formula that builds grammr from source using the git repository.

## Updating for New Versions

When creating a new release:

1. **Update the formula version:**
   - Edit `Formula/grammr.rb`
   - Update the `tag:` value to the new version (e.g., `"v1.1.0"`)

2. **Update the install script (optional):**
   - Edit `install.sh`
   - Update the default `VERSION` variable if needed

3. **Commit and push:**
   ```bash
   git add Formula/grammr.rb install.sh
   git commit -m "Update Homebrew formula for v1.1.0"
   git push
   ```

4. **Create and push the git tag:**
   ```bash
   git tag v1.1.0
   git push origin v1.1.0
   ```

## Installation Methods

Users can install grammr via Homebrew using:

**Method 1: Installation script (recommended)**
```bash
curl -fsSL https://raw.githubusercontent.com/maximbilan/grammr/main/install.sh | bash
```

**Method 2: Direct formula URL**
```bash
brew install --build-from-source https://raw.githubusercontent.com/maximbilan/grammr/v1.0.0/Formula/grammr.rb
```

## Testing the Formula Locally

To test the formula before pushing:

```bash
brew install --build-from-source Formula/grammr.rb
```

Note: This requires the formula to reference a tag that exists in the repository.
