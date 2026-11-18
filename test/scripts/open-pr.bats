#!/usr/bin/env bats
# SPDX-FileCopyrightText: 2026 The provider-stripe authors
#
# SPDX-License-Identifier: Apache-2.0

setup() {
  ROOT=$(cd "$BATS_TEST_DIRNAME/../.." && pwd)
  mkdir "$BATS_TEST_TMPDIR/bin"
  PATH=$BATS_TEST_TMPDIR/bin:$PATH
  CALLS=$BATS_TEST_TMPDIR/calls
  : > "$CALLS"
  export GH_TOKEN=token GITHUB_REPOSITORY=owner/repo
  export GIT_CONFIG_GLOBAL=$BATS_TEST_TMPDIR/gitconfig
  git config --global user.name test
  git config --global user.email test@example.org
  git config --global url."$BATS_TEST_TMPDIR/remote.git".insteadOf https://x-access-token:token@github.com/owner/repo.git
  git init -q --bare "$BATS_TEST_TMPDIR/remote.git"
  git init -q -b main "$BATS_TEST_TMPDIR/repo"
  cd "$BATS_TEST_TMPDIR/repo"
  mkdir openapi
  echo v2600 > openapi/VERSION
  git add openapi
  git commit -qm init
  echo v2700 > openapi/VERSION
  git diff --binary > "$BATS_TEST_TMPDIR/changes.patch"
  git checkout -q openapi/VERSION
}

open_prs() {
  echo "$1" > "$BATS_TEST_TMPDIR/prs.json"
  printf '#!/usr/bin/env bash\necho "gh $*" >> "%s"\n%s\n' "$CALLS" "case \"\$1 \$2\" in
    'pr list') cat '$BATS_TEST_TMPDIR/prs.json' ;;
    'pr create') echo https://github.com/owner/repo/pull/9 ;;
  esac" > "$BATS_TEST_TMPDIR/bin/gh"
  chmod +x "$BATS_TEST_TMPDIR/bin/gh"
}

@test "pushes the spec branch and opens a PR when none is open" {
  open_prs '[]'
  run "$ROOT/hack/open-pr.sh" v2700 "$BATS_TEST_TMPDIR/changes.patch" "spec body"
  [ "$status" -eq 0 ]
  [ "$(git -C "$BATS_TEST_TMPDIR/remote.git" show spec/v2700:openapi/VERSION)" = v2700 ]
  grep -qxF "gh pr create --base main --head spec/v2700 --title feat: track Stripe spec v2700 --body spec body" "$CALLS"
  ! grep -q "pr merge\|pr close" "$CALLS"
}

@test "force-updates the spec branch without opening a PR when one is open" {
  git switch -qc spec/v2700
  echo stale > openapi/VERSION
  git commit -qam stale
  git push -q https://x-access-token:token@github.com/owner/repo.git spec/v2700
  git switch -q main
  open_prs '[{"number":5,"headRefName":"spec/v2700","url":"https://github.com/owner/repo/pull/5"}]'
  run "$ROOT/hack/open-pr.sh" v2700 "$BATS_TEST_TMPDIR/changes.patch" "spec body"
  [ "$status" -eq 0 ]
  [ "$(git -C "$BATS_TEST_TMPDIR/remote.git" show spec/v2700:openapi/VERSION)" = v2700 ]
  ! grep -q "pr create\|pr close" "$CALLS"
}

@test "closes superseded spec PRs with a link to the current one, leaving other PRs open" {
  open_prs '[{"number":3,"headRefName":"spec/v2650","url":"https://github.com/owner/repo/pull/3"},
    {"number":4,"headRefName":"feature/x","url":"https://github.com/owner/repo/pull/4"}]'
  run "$ROOT/hack/open-pr.sh" v2700 "$BATS_TEST_TMPDIR/changes.patch" "spec body"
  [ "$status" -eq 0 ]
  grep -qxF "gh pr close 3 --comment Superseded by https://github.com/owner/repo/pull/9" "$CALLS"
  ! grep -q "pr close 4" "$CALLS"
}
