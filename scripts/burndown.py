import re
import datetime
import os
import matplotlib.pyplot as plt
import matplotlib.dates as mdates

def parse_todo_md(file_path, default_date):
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    current_date = default_date
    added_by_date = {}
    completed_by_date = {}

    for line in content.split('\n'):
        header_match = re.search(r'###.*\((\d{1,2})\.(\d{1,2})\)', line)
        if header_match:
            month = int(header_match.group(1))
            day = int(header_match.group(2))
            try:
                current_date = datetime.date(default_date.year, month, day)
            except ValueError:
                current_date = default_date
            continue

        if not line.strip().startswith('- ['):
            continue

        point_match = re.search(r'\((\d+)点\)', line)
        if not point_match:
            continue

        points = int(point_match.group(1))
        added_by_date[current_date] = added_by_date.get(current_date, 0) + points

        if line.strip().startswith('- [x]'):
            date_match = re.search(r'\(完成于: (\d{4}-\d{2}-\d{2})\)', line)
            if date_match:
                completed_date = datetime.datetime.strptime(date_match.group(1), '%Y-%m-%d').date()
            else:
                completed_date = current_date
            completed_by_date[completed_date] = completed_by_date.get(completed_date, 0) + points

    return added_by_date, completed_by_date

def generate_burndown_chart(added_by_date, completed_by_date, start_date_str, end_date_str, output_path):
    start_date = datetime.datetime.strptime(start_date_str, '%Y-%m-%d').date()
    end_date = datetime.datetime.strptime(end_date_str, '%Y-%m-%d').date()
    dates = [start_date + datetime.timedelta(days=i) for i in range((end_date - start_date).days + 1)]

    daily_added = [added_by_date.get(date, 0) for date in dates]
    daily_completed = [completed_by_date.get(date, 0) for date in dates]

    scope_points = []
    completed_points = []
    remaining_points = []
    scope_acc = 0
    completed_acc = 0
    for i in range(len(dates)):
        scope_acc += daily_added[i]
        completed_acc += daily_completed[i]
        scope_points.append(scope_acc)
        completed_points.append(completed_acc)
        remaining_points.append(scope_acc - completed_acc)

    final_scope = scope_points[-1] if scope_points else 0
    ideal_burndown = [final_scope - (final_scope / (len(dates) - 1)) * i for i in range(len(dates))] if len(dates) > 1 else [final_scope]

    today = datetime.date.today()
    actual_dates = []
    actual_remaining = []
    for i, date in enumerate(dates):
        if date > today:
            break
        actual_dates.append(date)
        actual_remaining.append(remaining_points[i])

    plt.style.use('seaborn-v0_8-darkgrid')

    plt.rcParams['font.sans-serif'] = ['SimHei']
    plt.rcParams['axes.unicode_minus'] = False
    fig, ax = plt.subplots(figsize=(12, 7))

    ax.plot(dates, ideal_burndown, linestyle='--', color='green', label='理想燃尽线(按最终范围)')
    ax.plot(dates, scope_points, linestyle=':', color='gray', label='范围(累计新增点数)')
    ax.plot(actual_dates, actual_remaining, marker='o', linestyle='-', color='dodgerblue', label='实际剩余点数')

    ax.set_title('NeoCode MVP Burndown Chart (Week 2)', fontsize=18, pad=20)
    ax.set_xlabel('日期', fontsize=12, labelpad=15)
    ax.set_ylabel('点数', fontsize=12, labelpad=15)
    ax.legend()
    ax.grid(True, which='both', linestyle='--', linewidth=0.5)

    ax.xaxis.set_major_formatter(mdates.DateFormatter('%m-%d'))
    ax.xaxis.set_major_locator(mdates.DayLocator())
    plt.gcf().autofmt_xdate()

    plt.tight_layout()
    plt.savefig(output_path, dpi=150)
    print(f"Burndown chart saved to {output_path}")

if __name__ == '__main__':
    script_dir = os.path.dirname(os.path.abspath(__file__))
    todo_file = os.path.abspath(os.path.join(script_dir, '..', 'docs', 'todo.md'))
    output_image = os.path.abspath(os.path.join(script_dir, '..', 'docs', 'burndown.png'))
    start_date = '2026-03-23'
    end_date = '2026-03-29'

    default_date = datetime.datetime.strptime(start_date, '%Y-%m-%d').date()
    added_by_date, completed_by_date = parse_todo_md(todo_file, default_date)
    generate_burndown_chart(added_by_date, completed_by_date, start_date, end_date, output_image)