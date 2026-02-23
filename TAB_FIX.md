# Tab样式修复说明

## 问题描述
历史记录下面的页签看不到了，还错位

## 修复内容

### 1. 添加 `!important` 确保样式生效
```css
padding: 0 20px !important;
```
- 防止Element Plus默认样式覆盖

### 2. 设置最小宽度防止标签过窄
```css
min-width: 80px;
```
- 确保每个Tab都有足够的显示空间

### 3. 文字居中对齐
```css
text-align: center;
```
- 让Tab文字在标签内居中显示

### 4. 使用Flex布局修复错位
```css
display: flex;
align-items: center;
```
- 确保Tab导航栏内部元素正确对齐

## 完整样式

```css
.status-tabs {
  margin-bottom: 16px;
}

.status-tabs :deep(.el-tabs__item) {
  padding: 0 20px !important;
  font-weight: 500;
  height: 36px;
  line-height: 36px;
  min-width: 80px;
  text-align: center;
}

.status-tabs :deep(.el-tabs__nav) {
  border-radius: 6px;
  background: #f5f7fa;
  padding: 4px;
  height: 36px;
  display: flex;
  align-items: center;
}

.status-tabs :deep(.el-tabs__item.is-active) {
  background: #409eff;
  color: #fff;
  border-radius: 4px;
}

.status-tabs :deep(.el-tabs__item:hover:not(.is-active)) {
  background: #e8f0fe;
  color: #409eff;
}
```

## 样式特点

- **Tab高度**: 36px
- **激活Tab**: 蓝色背景 + 白色文字
- **悬停Tab**: 浅蓝色背景 + 蓝色文字
- **圆角设计**: 6px 外框 + 4px 内标签
- **标签数量**: 4个 (全部/进行中/已完成/失败)

## 查看效果

```bash
cd /home/luo/luoyi/remoteagent/src/frontend
npm run dev
```

然后访问分发中心页面的历史记录部分查看修复效果！
