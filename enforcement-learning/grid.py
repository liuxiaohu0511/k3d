import numpy as np
import random

# ============ 环境设置 ============

GRID_SIZE = 4
# 4x4的网格，一共16个格子，用(行,列)来表示位置，比如(0,0)是左上角

# 用字符画出地图，方便直观看
# S=起点  G=终点  X=障碍物  .=普通格子
GRID = [
    ['S', '.', '.', '.'],
    ['.', 'X', '.', 'X'],
    ['.', '.', '.', '.'],
    ['X', '.', '.', 'G']
]

START = (0, 0)   # 起点坐标：第0行第0列
GOAL = (3, 3)    # 终点坐标：第3行第3列
OBSTACLES = [(1, 1), (1, 3), (3, 0)]  
# 障碍物坐标列表，对应地图里那三个X
# 例子：(1,1) 意思是"第1行第1列"，掉进去要扣分，相当于走进了陷阱

ACTIONS = ['up', 'down', 'left', 'right']
# 四个方向，比一维例子多了两个选择

ACTION_MOVES = {
    'up':    (-1, 0),   # 行减1，往上挪
    'down':  (1, 0),    # 行加1，往下挪
    'left':  (0, -1),   # 列减1，往左挪
    'right': (0, 1),    # 列加1，往右挪
}
# 例子：如果你在(1,2)，选择'up'，那么新位置 = (1-1, 2+0) = (0,2)
# 这个字典就是把"方向的名字"翻译成"坐标该怎么变"

EPSILON = 0.9    # 90%概率走"当前认为最好"的方向，10%随便探索
ALPHA = 0.1      # 学习率：每次只修正10%的认知偏差
GAMMA = 0.9      # 折扣因子：未来的奖励打9折
EPISODES = 200   # 训练200轮（网格比一维复杂，需要更多轮次才能学好）

# ---- Q表：这次要记录"每个坐标"在"每个方向"上的价值 ----
# 用字典比数组更方便，因为key可以直接是坐标元组
q_table = {}
for row in range(GRID_SIZE):
    for col in range(GRID_SIZE):
        q_table[(row, col)] = {a: 0.0 for a in ACTIONS}
# 结果长这样：
# q_table[(0,0)] = {'up':0.0, 'down':0.0, 'left':0.0, 'right':0.0}
# q_table[(0,1)] = {'up':0.0, 'down':0.0, 'left':0.0, 'right':0.0}
# ... 每个格子都对应一个"四个方向价值多少"的小字典
# 就像给16个格子每个都发一张"东南西北打分表"，一开始全是0分


# ============ 判断某个位置是否合法 ============

def is_valid(pos):
    """检查这个坐标是否在网格内，且不是障碍物"""
    row, col = pos
    if row < 0 or row >= GRID_SIZE or col < 0 or col >= GRID_SIZE:
        return False
        # 例子：如果算出新坐标是(-1, 2)，说明想往上跑出网格外，不合法
    if pos in OBSTACLES:
        return False
        # 例子：如果新坐标正好是(1,1)，那是个障碍物，也不合法
    return True


# ============ 决策函数：这一步该往哪走？ ============

def choose_action(state):
    if random.uniform(0, 1) > EPSILON:
        return random.choice(ACTIONS)
        # 10%概率随便选一个方向探索
        # 例子：像蒙眼在房间里随便摸索，不一定按经验走
    else:
        q_values = q_table[state]
        # 取出当前格子对应的"四方向打分表"
        # 例子：q_values = {'up':0.2, 'down':0.5, 'left':0.1, 'right':0.3}
        return max(q_values, key=q_values.get)
        # 找出打分表里"值最大"的那个方向的名字
        # 例子：上面这组数据里down=0.5最大，就返回'down'
        # max(..., key=q_values.get) 的意思是"按照字典的值来比较，返回值最大的那个key"


# ============ 环境反馈函数 ============

def get_env_feedback(state, action):
    """根据当前坐标和选择的方向，计算下一个坐标和奖励"""
    move = ACTION_MOVES[action]
    # 例子：action='right' → move=(0,1)

    next_state = (state[0] + move[0], state[1] + move[1])
    # 把当前坐标和移动量相加，算出理论上的新坐标
    # 例子：state=(2,2), move=(0,1) → next_state=(2,3)

    if not is_valid(next_state):
        # 如果这个新坐标不合法（撞墙或者是障碍物）
        return state, -1
        # 那就"原地不动"（返回原来的state），并且扣1分作为惩罚
        # 例子：你撞到墙上，位置没变，但是很疼，扣分！

    if next_state == GOAL:
        return next_state, 10
        # 走到终点，给10分大奖励（网格更复杂，奖励也要设置得更"诱人"一些）

    return next_state, -0.1
    # 正常移动一步，扣一点点分（鼓励走最短路径，别磨蹭）


# ============ 训练主循环 ============

for episode in range(EPISODES):
    state = START
    # 每一局都从起点(0,0)重新出发

    step_count = 0
    max_steps = 100
    # 设置最大步数上限，防止极端情况下智能体在网格里死循环走不出来

    while state != GOAL and step_count < max_steps:
        action = choose_action(state)
        next_state, reward = get_env_feedback(state, action)

        # ---- 核心更新公式，逻辑和一维例子完全一样 ----
        old_value = q_table[state][action]
        # 我原本以为，在这个坐标往这个方向走，值多少分

        next_max = max(q_table[next_state].values())
        # 看看走到新坐标后，四个方向里最好的那个能打多少分
        # 例子：q_table[(2,3)] = {'up':0.3,'down':0.1,'left':0.2,'right':0.9}
        # 那next_max就是0.9（往right走最划算）

        new_value = old_value + ALPHA * (reward + GAMMA * next_max - old_value)
        # 更新公式跟一维例子一模一样：
        # 新认知 = 旧认知 + 学习率 × (真实情况 - 旧认知)
        # 真实情况 = 这一步奖励 + 打折后"下一格子最好的未来预期"

        q_table[state][action] = new_value
        # 把修正后的值存回记忆本

        state = next_state
        step_count += 1

    if (episode + 1) % 50 == 0:
        # 每50轮打印一次进度，不然200轮全打印屏幕会爆炸
        print(f"第{episode+1}轮，用了{step_count}步")


# ============ 训练完，看看学到的最优路径 ============

def show_optimal_path():
    state = START
    path = [state]
    visited = set()

    while state != GOAL:
        if state in visited:
            print("陷入循环，可能没训练好")
            break
        visited.add(state)

        action = max(q_table[state], key=q_table[state].get)
        # 直接选Q值最大的方向，不再探索，这叫"贪心执行"
        # 例子：训练好之后，q_table[(0,0)]里right的值最高，就一直选right

        move = ACTION_MOVES[action]
        next_state = (state[0] + move[0], state[1] + move[1])

        if not is_valid(next_state):
            print("走到非法位置，训练可能不充分")
            break

        state = next_state
        path.append(state)

    return path

print("\n训练后学到的最优路径：")
print(show_optimal_path())