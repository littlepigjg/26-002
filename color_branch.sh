#!/bin/bash

# ============================================
#  Git Worktree 分支创建脚本 - 第二级
#  在 XX_bugN 或 XX-CC-bugN 分支上运行
#  创建 XX_bugN_green 和 XX_bugN_red
#
#  文件规则：
#    green: 删除 red_green_test.go 和所有中文名称文件
#    red:   保留 red_green_test.go，删除所有中文名称文件
#
#  推送规则：
#    N <= 20: green 和 red 都推送到远程
#    N > 20: green 仅本地，red 推送到远程
# ============================================

if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "[ERROR] 当前目录不是 Git 仓库"
    exit 1
fi

ORIG_DIR="$(pwd)"
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"

# 匹配 bug 分支名 (支持 _bug 和 -bug 两种格式)
if [[ ! "$CURRENT_BRANCH" =~ [_-]bug([0-9]+)$ ]]; then
    echo "[ERROR] 当前分支 '$CURRENT_BRANCH' 不是 bug 分支"
    echo "        请在 XX_bugN 或 XX-CC-bugN 分支上运行此脚本"
    exit 1
fi

BUG_NUM="${BASH_REMATCH[1]}"

# 提取基础名称 (去掉 _bugN 或 -bugN 后缀)
if [[ "$CURRENT_BRANCH" =~ ^(.+)[_-]bug[0-9]+$ ]]; then
    BASE_NAME="${BASH_REMATCH[1]}"
else
    BASE_NAME="$CURRENT_BRANCH"
fi

PARENT_DIR="$(dirname "$ORIG_DIR")"

if [ "$BUG_NUM" -le 20 ]; then
    echo "ℹ Bug #$BUG_NUM <= 20，green 和 red 都推送到远程"
else
    echo "⚠ Bug #$BUG_NUM > 20，green 分支仅本地创建，不推送到远程"
fi

echo "当前分支: $CURRENT_BRANCH"
echo "Bug 编号: #$BUG_NUM"
echo "基础名称: $BASE_NAME"
echo
echo "将基于分支 '$CURRENT_BRANCH' 创建子分支："
echo "============================================================"

# 查找中文名称文件 (在给定目录下，返回文件路径列表)
find_chinese_files() {
    local dir="$1"
    find "$dir" -maxdepth 3 -type f 2>/dev/null | while IFS= read -r f; do
        local bname=$(basename "$f")
        if echo "$bname" | grep -qP '[\p{Han}]'; then
            echo "$f"
        fi
    done
}

# 删除中文名称文件并提交
remove_chinese_files_and_commit() {
    local dir="$1"
    local keep_test="$2"  # "true" 表示保留 red_green_test.go
    
    cd "$dir"
    
    local REMOVED=0
    local CN_FILES=$(find_chinese_files "$dir")
    
    if [ -n "$CN_FILES" ]; then
        while IFS= read -r f; do
            local bname=$(basename "$f")
            if [ "$keep_test" = "true" ] && [ "$bname" = "red_green_test.go" ]; then
                continue
            fi
            rm -f "$f"
            echo "  [REMOVE] $bname"
            ((REMOVED++))
        done <<< "$CN_FILES"
    fi
    
    git add -A
    if [ -n "$(git status --porcelain)" ]; then
        git commit -m "remove Chinese-named files (keep test: $keep_test)" 2>/dev/null
        echo "  [COMMIT] 已提交变更 (删除 ${REMOVED} 个文件)"
    else
        echo "  [INFO] 无需提交"
    fi
    
    return $REMOVED
}

# 设置 green 分支
setup_green_branch() {
    local dir="$1"
    cd "$dir"
    
    # 1. 删除 red_green_test.go
    if [ -f "red_green_test.go" ]; then
        rm -f red_green_test.go
        echo "  [REMOVE] red_green_test.go"
    fi
    
    # 2. 提交删除 test 文件
    git add -A
    if [ -n "$(git status --porcelain)" ]; then
        git commit -m "remove red_green_test.go for green branch" 2>/dev/null
        echo "  [COMMIT] 已提交变更"
    fi
    
    # 3. 删除中文名称文件 (不保留 test)
    remove_chinese_files_and_commit "$dir" "false"
    
    cd "$ORIG_DIR"
}

# 设置 red 分支
setup_red_branch() {
    local dir="$1"
    cd "$dir"
    
    # 1. 删除中文名称文件 (保留 red_green_test.go)
    remove_chinese_files_and_commit "$dir" "true"
    
    cd "$ORIG_DIR"
}

SUCCESS=0
FAIL=0
SKIP=0

COLORS=(green red)

for COLOR in "${COLORS[@]}"; do
    NEW_BRANCH="${BASE_NAME}_bug${BUG_NUM}_${COLOR}"
    NEW_DIR="${PARENT_DIR}/${NEW_BRANCH}"

    if git show-ref --verify --quiet "refs/heads/$NEW_BRANCH" 2>/dev/null; then
        if git worktree list 2>/dev/null | grep -q "$NEW_BRANCH"; then
            echo "[SKIP] 分支 '$NEW_BRANCH' 已存在且有 worktree，跳过"
            ((SKIP++))
            continue
        else
            echo "[INFO] 分支 '$NEW_BRANCH' 存在但无 worktree，重建 worktree"
            if ! git worktree add "$NEW_DIR" "$NEW_BRANCH"; then
                echo "✗ 重建 worktree 失败"
                ((FAIL++))
                continue
            fi
            echo "✓ worktree 重建成功，重新设置文件规则..."
            if [ "$COLOR" = "green" ]; then
                setup_green_branch "$NEW_DIR"
            else
                setup_red_branch "$NEW_DIR"
            fi
            ((SUCCESS++))
            continue
        fi
    fi

    if [ -d "$NEW_DIR" ]; then
        echo "[INFO] 目录 '$NEW_DIR' 已存在但分支不存在，删除残留目录后重建"
        rm -rf "$NEW_DIR"
    fi

    echo -n "[CREATE] $NEW_BRANCH ... "

    if ! git worktree add -b "$NEW_BRANCH" "$NEW_DIR" "$CURRENT_BRANCH"; then
        echo "✗ 创建失败"
        echo "  错误: 无法创建 worktree，请检查目录权限或 git 状态"
        ((FAIL++))
        continue
    fi

    echo "✓ 创建成功"
    
    # 应用文件规则
    echo "  设置文件规则..."
    if [ "$COLOR" = "green" ]; then
        setup_green_branch "$NEW_DIR"
    else
        setup_red_branch "$NEW_DIR"
    fi
    
    # 压缩历史为单个 commit
    echo "  压缩历史为单个 commit..."
    cd "$NEW_DIR"
    
    local COMMIT_MSG=""
    if [ "$COLOR" = "green" ]; then
        COMMIT_MSG="green branch: remove red_green_test.go and Chinese files"
    else
        COMMIT_MSG="red branch: keep red_green_test.go, remove Chinese files"
    fi
    
    # 压缩为单个 commit
    git checkout --orphan temp_squash_branch 2>/dev/null
    git add -A
    git commit -q -m "$COMMIT_MSG" 2>/dev/null
    git branch -D "$NEW_BRANCH" 2>/dev/null
    git branch -m "$NEW_BRANCH" 2>/dev/null
    cd "$ORIG_DIR"
    echo "  ✓ 已压缩历史为单个 commit"
    
    # 推送规则
    if [ "$BUG_NUM" -gt 20 ] && [ "$COLOR" = "green" ]; then
        echo "  ⚠ Bug #$BUG_NUM > 20, green 分支不推送到远程"
        ((SUCCESS++))
        continue
    fi
    
    echo -n "  推送到远程... "
    if git push --force origin "$NEW_BRANCH" 2>/dev/null; then
        echo "✓ 推送成功"
        ((SUCCESS++))
    else
        echo "⚠ 推送失败（可重新运行脚本重试）"
        ((SUCCESS++))
    fi
done

echo ""
echo "============================================================"
echo "完成！成功: ${SUCCESS}, 跳过: ${SKIP}, 失败: ${FAIL}"
echo
echo "📂 分支结构："
for COLOR in "${COLORS[@]}"; do
    NEW_BRANCH="${BASE_NAME}_bug${BUG_NUM}_${COLOR}"
    NEW_DIR="${PARENT_DIR}/${NEW_BRANCH}"
    if git show-ref --verify --quiet "refs/heads/$NEW_BRANCH" 2>/dev/null; then
        if [ "$BUG_NUM" -gt 20 ] && [ "$COLOR" = "green" ]; then
            echo "   ${NEW_DIR}/  (分支: $NEW_BRANCH) [仅本地]"
        else
            echo "   ${NEW_DIR}/  (分支: $NEW_BRANCH)"
        fi
    fi
done

echo ""
echo "📋 规则说明："
echo "  green 分支: 无 red_green_test.go，无中文名称文件"
echo "  red 分支:   有 red_green_test.go，无中文名称文件"
if [ "$BUG_NUM" -le 20 ]; then
    echo "  Bug <= 20: green 和 red 都推送到远程"
else
    echo "  Bug > 20: green 仅本地开发，red 可推送到远程"
fi
