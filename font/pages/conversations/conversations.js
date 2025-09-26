const app = getApp()

Page({
  data: {
    conversations: [],
    loading: false,
    swipedIndex: -1, // 当前左滑的索引
    touchStartX: 0,
    touchStartY: 0,
    currentTouchIndex: -1,
    tallCache: {} // 新增：按URL缓存是否为长图
  },

  onLoad() {
    this.loadConversations()
  },

  onShow() {
    // 页面显示时自动刷新会话列表
    this.loadConversations()
  },

  // 加载会话列表
  loadConversations() {
    if (!app.globalData.userInfo) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      // 停止下拉刷新
      wx.stopPullDownRefresh()
      return
    }

    this.setData({ loading: true })
    
    wx.request({
      url: app.globalData.baseUrl + '/api/conversations',
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      success: (res) => {
        if (res.statusCode === 200 && res.data.code === 200) {
          // 格式化时间并合并缓存长图标记
          const cache = this.data.tallCache || {}
          const conversations = res.data.data.conversations.map(item => ({
            ...item,
            last_time: this.formatTime(item.last_time),
            isTall: cache[item.character_avatar] || false
          }))
          
          const sorted = conversations.slice().sort((a, b) => {
            if (a.is_top === 'yes' && b.is_top !== 'yes') return -1;
            if (a.is_top !== 'yes' && b.is_top === 'yes') return 1;
            return new Date(b.updated_at) - new Date(a.updated_at);
          });
          this.setData({ conversations: sorted });
          
          console.log('会话列表加载成功，共', conversations.length, '条记录')
        } else {
          wx.showToast({
            title: res.data.message || '加载失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        console.error('加载会话列表失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      },
      complete: () => {
        this.setData({ loading: false })
        // 停止下拉刷新
        wx.stopPullDownRefresh()
      }
    })
  },

  // 会话头像图片加载回调：判断是否为长图（阈值1.4），并写入缓存
  onConversationAvatarLoad(e) {
    const index = Number(e.currentTarget.dataset.index)
    const url = e.currentTarget.dataset.url
    const { width, height } = e.detail || {}
    if (Number.isNaN(index) || !width || !height) return
    const isTall = height / width > 1.4
    const key = `conversations[${index}].isTall`
    const cache = { ...(this.data.tallCache || {}) }
    if (url) cache[url] = isTall
    this.setData({ [key]: isTall, tallCache: cache })
  },

  // 点击会话
  onConversationTap(e) {
    const conversation = e.currentTarget.dataset.conversation
    
    // 如果当前有左滑状态，先关闭左滑
    if (this.data.swipedIndex !== -1) {
      this.setData({
        swipedIndex: -1
      })
      return
    }
    
    // 跳转到聊天页面
    wx.navigateTo({
      url: `/pages/chat/chat?characterId=${conversation.character_id}&characterName=${conversation.character_name}&characterAvatar=${conversation.character_avatar}`
    })
  },

  // 格式化时间
  formatTime(timeStr) {
    if (!timeStr) return ''
    
    const date = new Date(timeStr)
    const now = new Date()
    const diff = now - date
    
    // 小于1分钟
    if (diff < 60000) {
      return '刚刚'
    }
    
    // 小于1小时
    if (diff < 3600000) {
      return Math.floor(diff / 60000) + '分钟前'
    }
    
    // 小于24小时
    if (diff < 86400000) {
      return Math.floor(diff / 3600000) + '小时前'
    }
    
    // 小于7天
    if (diff < 604800000) {
      return Math.floor(diff / 86400000) + '天前'
    }
    
    // 超过7天显示具体日期
    const year = date.getFullYear()
    const month = (date.getMonth() + 1).toString().padStart(2, '0')
    const day = date.getDate().toString().padStart(2, '0')
    
    if (year === now.getFullYear()) {
      return `${month}-${day}`
    } else {
      return `${year}-${month}-${day}`
    }
  },

  // 下拉刷新
  onPullDownRefresh() {
    console.log('开始下拉刷新...')
    this.loadConversations()
  },

  // 触摸开始
  onTouchStart(e) {
    const touch = e.touches[0]
    const index = e.currentTarget.dataset.index
    
    this.setData({
      touchStartX: touch.clientX,
      touchStartY: touch.clientY,
      currentTouchIndex: index
    })
  },

  // 触摸移动
  onTouchMove(e) {
    const touch = e.touches[0]
    const deltaX = touch.clientX - this.data.touchStartX
    const deltaY = touch.clientY - this.data.touchStartY
    
    // 判断是否为水平滑动且左滑距离足够
    if (Math.abs(deltaX) > Math.abs(deltaY) && deltaX < -60) {
      // 左滑超过60px，显示操作按钮
      if (this.data.currentTouchIndex !== -1 && this.data.swipedIndex !== this.data.currentTouchIndex) {
        this.setData({
          swipedIndex: this.data.currentTouchIndex
        })
      }
    }
  },

  // 触摸结束
  onTouchEnd(e) {
    // 重置触摸状态
    this.setData({
      currentTouchIndex: -1
    })
  },

  // 点击其他地方关闭左滑
  onTapOutside() {
    if (this.data.swipedIndex !== -1) {
      this.setData({
        swipedIndex: -1
      })
    }
  },

  // 置顶会话
  onPinConversation(e) {
    const index = e.currentTarget.dataset.index
    const conversation = this.data.conversations[index]
    
    // 切换置顶状态
    const isTop = conversation.is_top === 'yes'
    const newIsTop = !isTop
    
    this.pinConversation(conversation.character_id, newIsTop, index)
    
    // 关闭左滑状态
    this.setData({
      swipedIndex: -1
    })
  },

  // 调用后端置顶会话
  pinConversation(characterId, isTop, index) {
    if (!app.globalData.userInfo) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      return
    }

    wx.showLoading({
      title: isTop ? '置顶中...' : '取消置顶中...'
    })

    wx.request({
      url: app.globalData.baseUrl + '/api/conversations/pin',
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        character_id: characterId,
        is_top: isTop // 直接传布尔值
      },
      success: (res) => {
        wx.hideLoading()
        
        if (res.statusCode === 200 && res.data.code === 200) {
          // 操作成功后重新拉取会话列表，保证排序和置顶状态和后端一致
          this.loadConversations();
          wx.showToast({
            title: isTop ? '已置顶' : '已取消置顶',
            icon: 'success'
          })
        } else {
          wx.showToast({
            title: res.data.message || '操作失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        console.error('置顶会话失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      }
    })
  },

  // 删除会话
  onDeleteConversation(e) {
    const index = e.currentTarget.dataset.index
    const conversation = this.data.conversations[index]
    
    wx.showModal({
      title: '确认删除',
      content: `确定要删除与"${conversation.character_name}"的聊天记录吗？删除后无法恢复。`,
      confirmText: '删除',
      confirmColor: '#ff6b6b',
      success: (res) => {
        if (res.confirm) {
          this.deleteConversation(conversation.character_id, index)
        }
      }
    })
  },

  // 调用后端删除会话
  deleteConversation(characterId, index) {
    if (!app.globalData.userInfo) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      return
    }

    wx.showLoading({
      title: '删除中...'
    })

    wx.request({
      url: app.globalData.baseUrl + '/api/conversations/' + characterId,
      method: 'DELETE',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      success: (res) => {
        wx.hideLoading()
        
        if (res.statusCode === 200 && res.data.code === 200) {
          // 从本地数据中移除
          const conversations = [...this.data.conversations]
          conversations.splice(index, 1)
          this.setData({ conversations })
          
          wx.showToast({
            title: '删除成功',
            icon: 'success'
          })
        } else {
          wx.showToast({
            title: res.data.message || '删除失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        console.error('删除会话失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      }
    })
  }
}) 