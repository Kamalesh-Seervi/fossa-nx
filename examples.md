# FOSSA-NX Simplified Usage Examples

After the CLI restructuring, the `fossa` subcommand has been removed for a cleaner user experience.

## Before (Old Structure)
```bash
fossa-nx fossa --all --github \
  --github-token=YOUR_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo
```

## After (New Simplified Structure)
```bash
fossa-nx --all --github \
  --github-token=YOUR_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo
```

## Key Benefits
- ✅ **Shorter Commands**: Removed redundant `fossa` subcommand
- ✅ **Cleaner CLI**: Direct execution without nested commands
- ✅ **Same Functionality**: All features work exactly the same
- ✅ **GitHub Integration**: Still includes both issue creation AND commit status checks

## What GitHub Integration Provides
1. **Issue Creation**: Creates detailed GitHub issues for each vulnerability
2. **Commit Status Checks**: Automatically posts success/failure status to the current commit
   - Status context: `ci/fossa-{project-name}`
   - ✅ Success: No vulnerabilities found
   - ❌ Failure: Vulnerabilities found or scan errors

## Example Commands

### Basic Analysis
```bash
# Analyze all projects
fossa-nx --all

# Analyze specific project
fossa-nx --project=my-app

# Analyze affected projects between commits
fossa-nx --base=main --head=feature-branch
```

### With GitHub Integration
```bash
# GitHub.com
fossa-nx --all --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo

# GitHub Enterprise
fossa-nx --all --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo \
  --github-api-url=https://github.yourcompany.com/api/v3
```

### Advanced Usage
```bash
# Verbose logging with custom concurrency
fossa-nx --all --verbose --concurrent=8 --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo

# Include unmapped projects with timeout
fossa-nx --all --include-unmapped --timeout=60 --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo

# Analyze affected projects between commits with GitHub integration and commit status
fossa-nx --base=main --head=feature-branch --concurrent=5 --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo

# Full CI/CD example: affected projects with verbose logging and GitHub integration
fossa-nx --base=develop --head=HEAD --concurrent=5 --verbose --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=my-org \
  --github-repo=my-repo \
  --github-api-url=https://github.yourcompany.com/api/v3
```

## CI/CD Pipeline Integration

Perfect for continuous integration workflows where you want to analyze only the changes in a pull request:

```bash
# Typical CI/CD usage: analyze changes between base and current HEAD
fossa-nx --base=main --head=HEAD --concurrent=5 --github \
  --github-token=$GITHUB_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo

# For GitHub Enterprise in CI/CD
fossa-nx --base=develop --head=$GITHUB_SHA --concurrent=5 --verbose --github \
  --github-token=$GITHUB_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo \
  --github-api-url=https://github.yourcompany.com/api/v3

# Disable smart change detection (analyze all projects regardless of changes)
fossa-nx --base=main --head=HEAD --smart-changes=false --github \
  --github-token=$GITHUB_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo
```

### Smart Change Detection 🚀

**NEW FEATURE**: By default, fossa-nx now includes smart change detection that only analyzes projects with actual file changes, similar to how NX handles affected projects.

**Key Benefits:**
- ⚡ **Faster CI/CD**: Skips unchanged projects automatically
- 💰 **Cost Savings**: Reduces unnecessary FOSSA API calls
- 🎯 **Relevant Results**: Focus on projects that actually changed
- 🔧 **Configurable**: Can be disabled with `--smart-changes=false`

**How It Works:**
1. Compares files between `--base` and `--head` commits
2. Maps changed files to affected NX projects using project graph
3. Only runs FOSSA scans on projects with changes
4. Skipped projects show as successful with 0 vulnerabilities

### What Happens When This Runs:

1. **📊 Project Analysis**: Identifies projects affected between `main` and `feature-branch`
2. **🔄 Concurrent Processing**: Runs up to 5 FOSSA scans simultaneously 
3. **🐛 Vulnerability Detection**: Scans each affected project for security issues
4. **📝 GitHub Issues**: Creates detailed issues for each vulnerability found
5. **✅ Commit Status**: Posts success/failure status to the current commit
   - Shows up in PR status checks as `ci/fossa-{project-name}`
   - ✅ Green checkmark if no vulnerabilities
   - ❌ Red X if vulnerabilities found or scan errors

### Environment Variables for CI/CD:

```bash
export GITHUB_TOKEN=ghp_your_personal_access_token
export FOSSA_API_KEY=your_fossa_api_key

# Then run
fossa-nx --base=main --head=HEAD --concurrent=5 --github \
  --github-token=$GITHUB_TOKEN \
  --github-org=your-org \
  --github-repo=your-repo
```

## Quick Reference Commands

### The Exact Command You Requested:
```bash
# Analyze changes between commits with GitHub integration and 5 concurrent workers
fossa-nx --base=main --head=feature-branch --concurrent=5 --github \
  --github-token=ghp_xxxxxxxxxxxx \
  --github-org=your-org \
  --github-repo=your-repo
```

### Command Breakdown:
- `--base=main` - Compare against main branch
- `--head=feature-branch` - Analyze up to feature-branch 
- `--concurrent=5` - Run 5 parallel FOSSA scans
- `--github` - Enable GitHub integration
- `--github-token=...` - Your GitHub API token
- `--github-org=your-org` - Your GitHub organization
- `--github-repo=your-repo` - Your repository name

### Result:
✅ **Issues Created**: GitHub issues for each vulnerability  
✅ **Status Posted**: Commit status check on the HEAD commit  
✅ **Fast Processing**: 5 concurrent scans for speed  
✅ **Smart Analysis**: Only affected projects between commits
