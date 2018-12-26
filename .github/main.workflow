workflow "Test & build" {
  on = "push"
  resolves = ["TestResult"]
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
