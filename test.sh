#!/bin/bash

# Linux服务管理器测试脚本
# 提供多种测试模式和选项

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    cat << EOF
Linux服务管理器测试脚本

用法: $0 [选项]

选项:
  -h, --help              显示此帮助信息
  -a, --all              运行所有测试（包括集成测试）
  -u, --unit             只运行单元测试（默认）
  -i, --integration      运行集成测试
  -b, --benchmark        运行性能基准测试
  -c, --coverage         生成测试覆盖率报告
  -v, --verbose          显示详细输出
  -s, --short            运行快速测试（跳过长时间运行的测试）
  -r, --race             启用竞态检测
  --clean                清理测试生成的文件

示例:
  $0                     # 运行单元测试
  $0 -a                  # 运行所有测试
  $0 -c                  # 生成覆盖率报告
  $0 -b                  # 运行性能测试
  $0 -i -v               # 运行集成测试并显示详细输出
  $0 --clean             # 清理测试文件

EOF
}

# 默认参数
RUN_UNIT=true
RUN_INTEGRATION=false
RUN_BENCHMARK=false
GENERATE_COVERAGE=false
VERBOSE=false
SHORT=false
RACE=false
CLEAN=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -a|--all)
            RUN_UNIT=true
            RUN_INTEGRATION=true
            RUN_BENCHMARK=true
            shift
            ;;
        -u|--unit)
            RUN_UNIT=true
            RUN_INTEGRATION=false
            RUN_BENCHMARK=false
            shift
            ;;
        -i|--integration)
            RUN_INTEGRATION=true
            shift
            ;;
        -b|--benchmark)
            RUN_BENCHMARK=true
            shift
            ;;
        -c|--coverage)
            GENERATE_COVERAGE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -s|--short)
            SHORT=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        *)
            print_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 清理函数
cleanup() {
    print_info "清理测试文件..."
    rm -f coverage.out coverage.html
    rm -f cpu.prof mem.prof
    rm -f test_*.log
    find . -name "*.tmp" -delete 2>/dev/null || true
    print_success "清理完成"
}

# 如果只是清理，则执行清理并退出
if [[ "$CLEAN" == true ]]; then
    cleanup
    exit 0
fi

# 检查Go环境
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go未安装或不在PATH中"
        exit 1
    fi
    
    print_info "Go版本: $(go version)"
}

# 构建测试参数
build_test_args() {
    local args=""
    
    if [[ "$VERBOSE" == true ]]; then
        args="$args -v"
    fi
    
    if [[ "$SHORT" == true ]]; then
        args="$args -short"
    fi
    
    if [[ "$RACE" == true ]]; then
        args="$args -race"
    fi
    
    echo "$args"
}

# 运行单元测试
run_unit_tests() {
    print_info "运行单元测试..."
    
    local args=$(build_test_args)
    local coverage_args=""
    
    if [[ "$GENERATE_COVERAGE" == true ]]; then
        coverage_args="-coverprofile=coverage.out -covermode=atomic"
    fi
    
    if go test $args $coverage_args ./pkg/... ./internal/... ./cmd/...; then
        print_success "单元测试通过"
        
        if [[ "$GENERATE_COVERAGE" == true ]]; then
            generate_coverage_report
        fi
    else
        print_error "单元测试失败"
        return 1
    fi
}

# 运行集成测试
run_integration_tests() {
    print_info "运行集成测试..."
    
    local args=$(build_test_args)
    
    # 检查是否有root权限（某些集成测试需要）
    if [[ $EUID -eq 0 ]]; then
        print_warning "以root权限运行集成测试"
    else
        print_warning "未以root权限运行，某些集成测试可能被跳过"
    fi
    
    if go test $args -tags=integration ./test/...; then
        print_success "集成测试通过"
    else
        print_error "集成测试失败"
        return 1
    fi
}

# 运行性能测试
run_benchmark_tests() {
    print_info "运行性能基准测试..."
    
    local bench_args="-bench=. -benchmem"
    
    if [[ "$VERBOSE" == true ]]; then
        bench_args="$bench_args -v"
    fi
    
    # 运行基准测试并保存结果
    go test $bench_args ./pkg/... ./internal/... ./cmd/... > benchmark_results.txt 2>&1
    
    if [[ $? -eq 0 ]]; then
        print_success "性能测试完成"
        print_info "结果已保存到 benchmark_results.txt"
        
        # 显示简要结果
        if [[ "$VERBOSE" == true ]]; then
            print_info "性能测试结果摘要:"
            grep "Benchmark" benchmark_results.txt | head -10
        fi
    else
        print_error "性能测试失败"
        cat benchmark_results.txt
        return 1
    fi
}

# 生成覆盖率报告
generate_coverage_report() {
    if [[ ! -f coverage.out ]]; then
        print_warning "未找到覆盖率文件"
        return
    fi
    
    print_info "生成覆盖率报告..."
    
    # 显示总体覆盖率
    local total_coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    print_info "总体测试覆盖率: $total_coverage"
    
    # 生成HTML报告
    go tool cover -html=coverage.out -o coverage.html
    print_success "HTML覆盖率报告已生成: coverage.html"
    
    # 显示文件级覆盖率
    if [[ "$VERBOSE" == true ]]; then
        print_info "文件级覆盖率详情:"
        go tool cover -func=coverage.out
    fi
}

# 运行静态分析
run_static_analysis() {
    print_info "运行静态分析..."
    
    # 检查代码格式
    print_info "检查代码格式..."
    if ! gofmt -l . | grep -q .; then
        print_success "代码格式检查通过"
    else
        print_warning "发现格式问题:"
        gofmt -l .
    fi
    
    # 运行go vet
    print_info "运行go vet..."
    if go vet .; then
        print_success "go vet检查通过"
    else
        print_error "go vet检查失败"
        return 1
    fi
    
    # 检查go mod
    print_info "检查go mod..."
    if go mod tidy && git diff --quiet go.mod go.sum; then
        print_success "go mod检查通过"
    else
        print_warning "go.mod或go.sum可能需要更新"
    fi
}

# 生成测试报告
generate_test_report() {
    local report_file="test_report_$(date +%Y%m%d_%H%M%S).md"
    
    print_info "生成测试报告: $report_file"
    
    cat > "$report_file" << EOF
# Linux服务管理器测试报告

**测试时间**: $(date)
**Go版本**: $(go version)
**操作系统**: $(uname -a)

## 测试执行情况

EOF

    if [[ "$RUN_UNIT" == true ]]; then
        echo "- ✅ 单元测试: 已执行" >> "$report_file"
    fi
    
    if [[ "$RUN_INTEGRATION" == true ]]; then
        echo "- ✅ 集成测试: 已执行" >> "$report_file"
    fi
    
    if [[ "$RUN_BENCHMARK" == true ]]; then
        echo "- ✅ 性能测试: 已执行" >> "$report_file"
    fi
    
    if [[ "$GENERATE_COVERAGE" == true && -f coverage.out ]]; then
        local coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
        echo "" >> "$report_file"
        echo "## 测试覆盖率" >> "$report_file"
        echo "" >> "$report_file"
        echo "**总体覆盖率**: $coverage" >> "$report_file"
    fi
    
    if [[ -f benchmark_results.txt ]]; then
        echo "" >> "$report_file"
        echo "## 性能测试结果" >> "$report_file"
        echo "" >> "$report_file"
        echo "\`\`\`" >> "$report_file"
        grep "Benchmark" benchmark_results.txt | head -10 >> "$report_file"
        echo "\`\`\`" >> "$report_file"
    fi
    
    print_success "测试报告已生成: $report_file"
}

# 主执行流程
main() {
    print_info "开始Linux服务管理器测试..."
    print_info "测试配置: 单元测试=$RUN_UNIT, 集成测试=$RUN_INTEGRATION, 性能测试=$RUN_BENCHMARK, 覆盖率=$GENERATE_COVERAGE"
    
    # 检查环境
    check_go
    
    # 运行静态分析
    run_static_analysis
    
    # 运行测试
    local failed=false
    
    if [[ "$RUN_UNIT" == true ]]; then
        if ! run_unit_tests; then
            failed=true
        fi
    fi
    
    if [[ "$RUN_INTEGRATION" == true ]]; then
        if ! run_integration_tests; then
            failed=true
        fi
    fi
    
    if [[ "$RUN_BENCHMARK" == true ]]; then
        if ! run_benchmark_tests; then
            failed=true
        fi
    fi
    
    # 生成报告
    generate_test_report
    
    # 最终结果
    if [[ "$failed" == true ]]; then
        print_error "某些测试失败"
        exit 1
    else
        print_success "所有测试通过!"
    fi
}

# 捕获中断信号进行清理
trap cleanup EXIT

# 执行主函数
main "$@"