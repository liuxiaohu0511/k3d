import gymnasium as gym          # 强化学习标准环境库，CartPole就内置在里面
import torch
import torch.nn as nn
import torch.optim as optim
import numpy as np
import random
from collections import deque    # 双端队列，用来实现经验回放池

# ============ 环境 ============
env = gym.make('CartPole-v1')
STATE_DIM = env.observation_space.shape[0]   # 4：车位置、车速度、杆角度、杆角速度
ACTION_DIM = env.action_space.n              # 2：向左推 / 向右推

# ============ 超参数 ============
GAMMA = 0.99              # 折扣因子，跟之前一样
LR = 1e-3                 # 神经网络的学习率
BATCH_SIZE = 64           # 每次训练从回放池里抽64条经验出来学
BUFFER_SIZE = 10000       # 回放池最多存1万条经验，存满了就把最老的挤掉
EPSILON_START = 1.0       # 一开始100%随机探索（完全没学过，瞎走）
EPSILON_END = 0.01        # 最终降到1%随机
EPSILON_DECAY = 500       # 大约500步内完成衰减
TARGET_UPDATE_FREQ = 10   # 每10局，把目标网络同步成主网络
EPISODES = 300

# ============ 神经网络：输入状态，输出每个动作的价值 ============
class QNetwork(nn.Module):
    def __init__(self, state_dim, action_dim):
        super().__init__()
        self.net = nn.Sequential(
            nn.Linear(state_dim, 128),   # 4个输入 → 128个隐藏神经元
            nn.ReLU(),
            nn.Linear(128, 128),
            nn.ReLU(),
            nn.Linear(128, action_dim)   # 128 → 2个输出，分别是"左"和"右"的价值
        )
    def forward(self, x):
        return self.net(x)
    # 例子：输入 [0.02, -0.89, 0.05, 1.2]（当前车杆状态）
    # 输出可能是 [0.31, 0.78] → 意思是"往左价值0.31，往右价值0.78"，选往右

# ============ 经验回放池 ============
class ReplayBuffer:
    def __init__(self, capacity):
        self.buffer = deque(maxlen=capacity)
    def push(self, s, a, r, s_next, done):
        self.buffer.append((s, a, r, s_next, done))
        # 存一条"发生过什么"的记录，就像写日记
    def sample(self, batch_size):
        batch = random.sample(self.buffer, batch_size)
        # 从所有日记里随机抽64条出来复习，而不是只看最近发生的
        s, a, r, s_next, done = zip(*batch)
        return (np.array(s), np.array(a), np.array(r), np.array(s_next), np.array(done))
    def __len__(self):
        return len(self.buffer)

# ============ 建两个网络：一个用来做决策，一个用来算"标准答案" ============
policy_net = QNetwork(STATE_DIM, ACTION_DIM)   # 主网络，实时更新
target_net = QNetwork(STATE_DIM, ACTION_DIM)   # 目标网络，隔一阵子才同步
target_net.load_state_dict(policy_net.state_dict())  # 一开始两者完全一样
target_net.eval()  # 目标网络不需要自己训练，只负责被动同步

optimizer = optim.Adam(policy_net.parameters(), lr=LR)
buffer = ReplayBuffer(BUFFER_SIZE)

# ============ ε-贪心选动作（逻辑跟Q-table版一模一样） ============
def choose_action(state, epsilon):
    if random.random() < epsilon:
        return env.action_space.sample()   # 随机探索
    with torch.no_grad():
        state_t = torch.FloatTensor(state).unsqueeze(0)
        q_values = policy_net(state_t)
        return q_values.argmax().item()    # 选网络认为价值最高的动作

# ============ 训练一步：这是DQN的核心，对应之前Q表更新公式 ============
def train_step():
    if len(buffer) < BATCH_SIZE:
        return  # 经验还不够一批，先不训练
    
    states, actions, rewards, next_states, dones = buffer.sample(BATCH_SIZE)
    states_t = torch.FloatTensor(states)
    actions_t = torch.LongTensor(actions)
    rewards_t = torch.FloatTensor(rewards)
    next_states_t = torch.FloatTensor(next_states)
    dones_t = torch.FloatTensor(dones)

    # 用主网络算"我原本以为这些动作值多少分"（对应Q-table里的q_predict）
    q_values = policy_net(states_t).gather(1, actions_t.unsqueeze(1)).squeeze(1)

    # 用目标网络算"更真实的估计"（对应Q-table里的q_target）
    with torch.no_grad():
        next_q_values = target_net(next_states_t).max(1)[0]
        target = rewards_t + GAMMA * next_q_values * (1 - dones_t)
        # (1 - dones) 的作用：如果这一步游戏已经结束了，就不该再加未来的奖励

    # 用均方误差衡量"我猜的"和"更真实的"之间差多少，然后反向传播修正网络
    loss = nn.functional.mse_loss(q_values, target)
    optimizer.zero_grad()
    loss.backward()
    optimizer.step()

# ============ 训练主循环 ============
epsilon = EPSILON_START
total_steps = 0
episode_rewards = []

for episode in range(EPISODES):
    state, _ = env.reset()
    episode_reward = 0
    done = False

    while not done:
        action = choose_action(state, epsilon)
        next_state, reward, terminated, truncated, _ = env.step(action)
        done = terminated or truncated   # 杆倒了 或者 超过500步上限

        buffer.push(state, action, reward, next_state, done)  # 记一笔经验
        state = next_state
        episode_reward += reward

        train_step()   # 每一步都顺便训练一下网络
        total_steps += 1
        epsilon = max(EPSILON_END, EPSILON_START - total_steps / EPSILON_DECAY)

    episode_rewards.append(episode_reward)

    if episode % TARGET_UPDATE_FREQ == 0:
        target_net.load_state_dict(policy_net.state_dict())  # 目标网络"追上"主网络

    if (episode + 1) % 20 == 0:
        avg = np.mean(episode_rewards[-20:])
        print(f"第{episode+1}局，最近20局平均得分：{avg:.1f}，探索率：{epsilon:.2f}")