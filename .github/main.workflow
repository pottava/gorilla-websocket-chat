workflow "Test & build" {
  on = "push"
  resolves = ["TestResult"]
}

workflow "Release a new version" {
  on = "release"
  resolves = ["ReleaseResult"]
}

action "Branch" {
  uses = "actions/bin/filter@master"
  args = "branch"
}

action "Deps" {
  uses = "supinf/github-actions/go/deps@master"
  env = {
    SRC_DIR = "server/"
  }
}

action "Lint" {
  needs = ["Branch", "Deps"]
  uses = "supinf/github-actions/go/lint@master"
  env = {
    SRC_DIR = "server/"
  }
}

action "Test" {
  needs = ["Deps"]
  uses = "supinf/github-actions/go/test@master"
  env = {
    SRC_DIR = "server/"
  }
}

action "Build" {
  needs = ["Deps"]
  uses = "supinf/github-actions/go/build@master"
  env = {
    SRC_DIR = "server/"
  }
}

action "TestResult" {
  needs = ["Lint", "Test", "Build"]
  uses = "actions/bin/debug@master"
}

action "Tags" {
  uses = "actions/bin/filter@master"
  args = "tag v*"
}

action "ReleaseBuild" {
  needs = ["Deps"]
  uses = "supinf/github-actions/go/build@master"
  env = {
    SRC_DIR = "server/"
    BUILD_OPTIONS = "-X main.version=${version}-${GITHUB_SHA:0:7} -X main.date=$(date +%Y-%m-%d --utc)"
  }
}

action "Release" {
  needs = ["Tags", "ReleaseBuild"]
  uses = "supinf/github-actions/github/release@master"
  env = {
    ARTIFACT_DIR = "server/dist/"
  }
  secrets = ["GITHUB_TOKEN"]
}

action "ReleaseResult" {
  needs = ["Release"]
  uses = "actions/bin/debug@master"
}
