import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import seaborn as sns

# Set style
plt.style.use('bmh')
sns.set_theme(style="whitegrid")

# Data
hardware = ['Leaseweb (SSD)', 'Hetzner (HDD)', 'Macbook (NVME)']
test_size = [21, 14, 30]  # GiB

# Times in seconds
mkbrr = [7.24, 41.02, 9.71]
mktorrent = [45.41, 68.17, 10.90]
torrenttools = [9.07, 47.97, np.nan]
torf = [8.85, 58.19, 9.78]

# Create DataFrame
df = pd.DataFrame({
    'Hardware': hardware,
    'Test Size (GiB)': test_size,
    'mkbrr': mkbrr,
    'mktorrent': mktorrent,
    'torrenttools': torrenttools,
    'torf': torf
})

plt.rcParams['figure.figsize'] = (10, 6)
plt.rcParams['font.size'] = 10

# Create horizontal bar plot
fig, ax = plt.subplots()
y = np.arange(len(hardware))
width = 0.2

ax.barh(y - width*1.5, mkbrr, width, label='mkbrr', color='#2ecc71')
ax.barh(y - width/2, mktorrent, width, label='mktorrent', color='#e74c3c')
ax.barh(y + width/2, torrenttools, width, label='torrenttools', color='#3498db')
ax.barh(y + width*1.5, torf, width, label='torf', color='#f1c40f')

# Customize plot
ax.set_xlabel('Time (seconds)')
ax.set_title('Torrent Creation Performance Comparison')
ax.set_yticks(y)
ax.set_yticklabels(hardware)
ax.legend(bbox_to_anchor=(1.02, 1), loc='upper left')

# Add test size labels
for i, size in enumerate(test_size):
    ax.text(1, i, f'{size} GiB', ha='left', va='center')

# Adjust layout
plt.tight_layout()

# Save plot
plt.savefig('benchmark_comparison.png', dpi=300, bbox_inches='tight')

# Create speed comparison plot
speed_vs_mktorrent = [6.3, 1.7, 1.1]
speed_vs_torrenttools = [1.3, 1.2, np.nan]
speed_vs_torf = [1.2, 1.4, 1.0]

fig, ax = plt.subplots()
y = np.arange(len(hardware))
width = 0.25

ax.barh(y - width, speed_vs_mktorrent, width, label='vs mktorrent', color='#e74c3c')
ax.barh(y, speed_vs_torrenttools, width, label='vs torrenttools', color='#3498db')
ax.barh(y + width, speed_vs_torf, width, label='vs torf', color='#f1c40f')

# Customize plot
ax.set_xlabel('Speed Multiplier (×)')
ax.set_title('mkbrr Speed Comparison')
ax.set_yticks(y)
ax.set_yticklabels(hardware)
ax.legend(bbox_to_anchor=(1.02, 1), loc='upper left')

# Add vertical line at 1.0
ax.axvline(x=1.0, color='black', linestyle='--', alpha=0.3)

# Adjust layout
plt.tight_layout()

# Save plot
plt.savefig('speed_comparison.png', dpi=300, bbox_inches='tight')

# Create consistency plot (standard deviation percentages)
std_mkbrr = [0.25, 2.39, 3.66]
std_mktorrent = [0.36, 39.10, 6.43]
std_torrenttools = [1.02, 22.00, np.nan]
std_torf = [0.87, 9.95, 7.66]

fig, ax = plt.subplots()
y = np.arange(len(hardware))
width = 0.2

ax.barh(y - width*1.5, std_mkbrr, width, label='mkbrr', color='#2ecc71')
ax.barh(y - width/2, std_mktorrent, width, label='mktorrent', color='#e74c3c')
ax.barh(y + width/2, std_torrenttools, width, label='torrenttools', color='#3498db')
ax.barh(y + width*1.5, std_torf, width, label='torf', color='#f1c40f')

# Customize plot
ax.set_xlabel('Standard Deviation (%)')
ax.set_title('Performance Consistency Comparison')
ax.set_yticks(y)
ax.set_yticklabels(hardware)
ax.legend(bbox_to_anchor=(1.02, 1), loc='upper left')

# Adjust layout
plt.tight_layout()

# Save plot
plt.savefig('consistency_comparison.png', dpi=300, bbox_inches='tight') 