#!/usr/bin/env python3
"""
覆盖率检查脚本
排除不可测试代码（main.go, Start, Stop），计算可测试代码覆盖率
"""

import sys

def check_coverage(coverage_file='coverage.out', threshold=95.0):
    total_statements = 0
    total_covered = 0
    excluded_count = 0
    
    try:
        with open(coverage_file) as f:
            for line in f:
                if line.startswith('mode:'):
                    continue
                parts = line.strip().split()
                if len(parts) >= 3:
                    file = parts[0]
                    func = parts[1].split(':')[-1] if ':' in parts[1] else ''
                    count = parts[-1]
                    
                    # 排除不可测试代码
                    is_untestable = 'main.go' in file or 'Start' in func or 'Stop' in func
                    
                    if count.isdigit():
                        if is_untestable:
                            excluded_count += 1
                        else:
                            total_statements += 1
                            if int(count) > 0:
                                total_covered += 1
    except FileNotFoundError:
        print(f"错误: 找不到覆盖率文件 {coverage_file}")
        sys.exit(1)
    
    if total_statements == 0:
        print("错误: 没有可测试的语句")
        sys.exit(1)
    
    coverage_pct = total_covered * 100.0 / total_statements
    
    print("=" * 50)
    print("dehub-server 覆盖率报告")
    print("=" * 50)
    print(f"可测试代码覆盖率: {coverage_pct:.1f}%")
    print(f"已覆盖: {total_covered}/{total_statements} 语句")
    print(f"排除不可测试: {excluded_count} 语句")
    print(f"目标阈值: {threshold}%")
    print("=" * 50)
    
    if coverage_pct >= threshold:
        print(f"✓ 通过: 覆盖率 {coverage_pct:.1f}% >= {threshold}%")
        return 0
    else:
        print(f"✗ 失败: 覆盖率 {coverage_pct:.1f}% < {threshold}%")
        print(f"  还需覆盖: {int((threshold - coverage_pct) * total_statements / 100)} 语句")
        return 1

if __name__ == '__main__':
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument('--file', default='coverage.out', help='覆盖率文件')
    parser.add_argument('--threshold', type=float, default=95.0, help='覆盖率阈值')
    args = parser.parse_args()
    
    sys.exit(check_coverage(args.file, args.threshold))
