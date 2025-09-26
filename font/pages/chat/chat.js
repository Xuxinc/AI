const app = getApp()

Page({
  data: {
    characterId: null,
    characterName: '',
    characterAvatar: '',
    userInfo: null,
    messages: [],
    inputMessage: '',
    scrollToMessage: '',
    showVoiceModal: false,
    loading: false,
    streamEnabled: true, // 默认开启流式传输
    isStreaming: false, // 是否正在流式传输
    currentStreamingMessage: null, // 当前正在流式传输的消息
    dialogId: null, // 存储对话ID
    selectedImages: [], // 选中的图片列表
    uploadedImageUrls: [], // 已上传的图片URL列表
    isTopAvatarTall: false, // 新增：顶部与AI头像是否为长图
  },

  onLoad(options) {
    this.setData({
      characterId: parseInt(options.characterId),
      characterName: options.characterName,
      characterAvatar: options.characterAvatar,
      userInfo: app.globalData.userInfo,
      streamEnabled: true // 默认开启流式传输
    })


    this.loadDialog()

    // 监听键盘高度变化 - 仅用于滚动到底部，不调整输入框位置
    wx.onKeyboardHeightChange((res) => {
      // 键盘弹出时滚动到底部
      if (res.height > 0) {
        setTimeout(() => {
          this.scrollToBottom()
        }, 100)
      }
    })
  },

  // 顶部头像加载完成，判断是否为长图（阈值1.4）
  onTopAvatarLoad(e) {
    const { width, height } = e.detail || {}
    if (!width || !height) return
    const isTall = height / width > 1.4
    if (isTall !== this.data.isTopAvatarTall) {
      this.setData({ isTopAvatarTall: isTall })
    }
  },

  // AI头像加载（沿用顶部头像的判断即可，此回调用于兜底刷新）
  onAiAvatarLoad(e) {
    if (this.data.isTopAvatarTall) return
    const { width, height } = e.detail || {}
    if (!width || !height) return
    const isTall = height / width > 1.4
    if (isTall) {
      this.setData({ isTopAvatarTall: true })
    }
  },

  // 页面渲染完成
  onReady() {
    // 页面渲染完成后再次确保滚动到底部
    setTimeout(() => {
      this.ensureScrollToBottom()
    }, 800)
  },

  // 加载对话
  loadDialog() {
    if (!app.globalData.userInfo) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      return
    }

    if (!app.globalData.token) {
      wx.showToast({
        title: '登录状态异常',
        icon: 'error'
      })
      return
    }

    // 防止重复请求
    if (this.data.loading) {
      console.log('正在加载中，跳过重复请求')
      return
    }

    console.log('开始加载对话 - 角色ID:', this.data.characterId, '用户信息:', app.globalData.userInfo, '流式传输:', this.data.streamEnabled)
    this.setData({ loading: true })
    
    wx.request({
      url: app.globalData.baseUrl + '/api/chat/dialog',
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        character_id: this.data.characterId
      },
      success: (res) => {
        console.log('对话加载响应:', res)
        if (res.statusCode === 200 && res.data.code === 200) {
          // 新增：存储dialog_id
          if (res.data.data && res.data.data.dialog_id) {
            this.setData({ dialogId: res.data.data.dialog_id })
          }
          
          // 安全处理消息数组
          const messages = res.data.data && res.data.data.messages ? res.data.data.messages : []
          
          console.log('从后端接收到的原始消息:', messages)
          
          // 过滤掉语音消息，只显示文字消息
          const filteredMessages = messages.filter(msg => {
            if (msg.is_voice === true || msg.is_voice === "yes") {
              return false
            }
            return true
          }).map(msg => {
            // 处理消息中的图片URL
            return this.processMessageImages(msg)
          })
          
          console.log('过滤前消息数量:', messages.length)
          console.log('过滤后消息数量:', filteredMessages.length)
          console.log('处理后的消息:', filteredMessages)
          
          // 调试：检查每个消息的图片信息
          filteredMessages.forEach((msg, index) => {
            console.log(`消息 ${index + 1}:`, {
              id: msg.id,
              role: msg.role,
              content: msg.content,
              picture_url: msg.picture_url,
              images: msg.images,
              hasImages: msg.images && msg.images.length > 0
            })
          })
          
          // 检查是否有本地的通话时长消息需要保留
          const lastMsg = wx.getStorageSync('lastCallDurationMsg');
          const lastDialogId = wx.getStorageSync('lastCallDialogId');
          let finalMessages = filteredMessages;
          
          if (lastMsg && lastDialogId == this.data.dialogId) {
            // 检查后端消息中是否已经包含通话时长消息
            const hasCallDurationMessage = filteredMessages.some(msg => 
              msg.content && msg.content.includes('通话时长:')
            );
            
            if (!hasCallDurationMessage) {
              // 如果后端没有通话时长消息，添加本地的
              const callDurationMsg = {
                id: 'temp_call_duration_' + Date.now(), // 临时ID，标识为通话时长消息
                content: lastMsg,
                is_voice: 'no',
                role: 'user',
                time: wx.getStorageSync('lastCallDurationTime') || new Date().toISOString()
              };
              finalMessages = [...filteredMessages, callDurationMsg];
              
              // 清除本地存储
              wx.removeStorageSync('lastCallDurationMsg');
              wx.removeStorageSync('lastCallDurationTime');
              wx.removeStorageSync('lastCallDialogId');
            } else {
              // 如果后端已有通话时长消息，清除本地存储
              wx.removeStorageSync('lastCallDurationMsg');
              wx.removeStorageSync('lastCallDurationTime');
              wx.removeStorageSync('lastCallDialogId');
            }
          }
          
          this.setData({
            messages: finalMessages,
            characterName: res.data.data && res.data.data.character ? res.data.data.character.name : this.data.characterName,
            characterAvatar: res.data.data && res.data.data.character ? res.data.data.character.avatar_url : this.data.characterAvatar
          })
          // 页面加载时强制滚动到底部
          this.forceScrollToBottom()
          // 额外确保滚动到底部
          setTimeout(() => {
            this.ensureScrollToBottom()
          }, 600)
          console.log('对话加载成功，消息数量:', filteredMessages.length)
        } else {
          console.error('对话加载失败:', res.data)
          wx.showToast({
            title: res.data.message || '加载对话失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        console.error('加载对话失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      },
      complete: () => {
        this.setData({ loading: false })
      }
    })
  },

  // 流式传输开关点击事件
  onStreamToggleTap() {
    const newStreamEnabled = !this.data.streamEnabled
    
    this.setData({
      streamEnabled: newStreamEnabled
    })
    
    // 显示提示
    const status = newStreamEnabled ? '开启' : '关闭'
    wx.showToast({
      title: `已${status}流式传输`,
      icon: 'success',
      duration: 1500
    })
  },


  // 角色头像点击
  onCharacterAvatarTap() {
    wx.navigateTo({
      url: `/pages/character-detail/character-detail?characterId=${this.data.characterId}`
    })
  },

  // 输入框变化
  onInputChange(e) {
    this.setData({
      inputMessage: e.detail.value
    })
    
    // 处理textarea高度变化，重新滚动到底部
    setTimeout(() => {
      this.scrollToBottom()
    }, 50)
  },

  // 输入框聚焦
  onInputFocus(e) {
    // 输入框聚焦时滚动到底部
    setTimeout(() => {
      this.scrollToBottom()
    }, 100)
  },

  // 输入框失焦
  onInputBlur(e) {
    // 可以在这里添加失焦时的处理逻辑
  },

  // 发送消息
  async sendMessage() {
    const message = this.data.inputMessage.trim()
    const hasImages = this.data.selectedImages.length > 0
    
    // 验证消息内容
    if (!message && !hasImages) {
      wx.showToast({
        title: '请输入消息或选择图片',
        icon: 'none',
        duration: 2000
      })
      return
    }

    if (!app.globalData.userInfo) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      return
    }

    // 如果有图片，先上传图片
    let imageUrls = []
    if (hasImages) {
      try {
        imageUrls = await this.uploadImages()
      } catch (error) {
        console.error('图片上传失败:', error)
        return
      }
    }

    // 清空输入和选中的图片
    this.setData({ 
      inputMessage: '',
      selectedImages: []
    })

    // 添加用户消息到列表（使用临时ID，稍后会被后端真实ID替换）
    const userMessage = {
      id: 'temp_' + Date.now(),
      role: 'user',
      content: message,
      images: imageUrls,
      time: new Date()
    }
    
    this.setData({
      messages: [...(this.data.messages || []), userMessage]
    })
    // 发送后立即滚动到底部
    this.ensureScrollToBottom()

    // 立即插入AI占位气泡，提升响应体验
    const tempAiMessage = {
      id: 'ai_temp_' + Date.now(),
      role: 'ai',
      content: '',
      time: new Date(),
      isStreaming: true
    }
    this.setData({
      messages: [...(this.data.messages || []), tempAiMessage],
      isStreaming: true
    })
    this.ensureScrollToBottom()

    // 根据流式传输设置选择发送方式
    if (this.data.streamEnabled) {
      this.sendStreamMessage(message, imageUrls)
    } else {
      this.sendNormalMessage(message, imageUrls)
    }
  },

  // 发送普通消息
  sendNormalMessage(message, imageUrls = []) {
    wx.request({
      url: app.globalData.baseUrl + '/api/chat/message',
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        character_id: this.data.characterId,
        message: message,
        image_urls: imageUrls
      },
      success: (res) => {
        if (res.statusCode === 200 && res.data.code === 200) {
          const userMessage = res.data.data.user_message
          const aiMessage = res.data.data.ai_message
          
          // 更新最后一条用户消息ID（从后向前查找最近的user）
          const messages = [...(this.data.messages || [])]
          let lastUserMessageIndex = -1
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === 'user') { lastUserMessageIndex = i; break }
          }
          if (lastUserMessageIndex !== -1) {
            messages[lastUserMessageIndex].id = userMessage.id
          }

          // 用AI占位气泡替换为真实内容（从后向前找到isStreaming的ai）
          let lastAiStreamingIndex = -1
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === 'ai' && messages[i].isStreaming) { lastAiStreamingIndex = i; break }
          }
          if (lastAiStreamingIndex !== -1) {
            messages[lastAiStreamingIndex] = {
              id: aiMessage.id,
              role: 'ai',
              content: aiMessage.content,
              picture_url: aiMessage.picture_url,
              time: new Date(aiMessage.time),
              isStreaming: false
            }
          } else {
            // 兜底：未找到占位气泡则直接追加
            messages.push({
              id: aiMessage.id,
              role: 'ai',
              content: aiMessage.content,
              picture_url: aiMessage.picture_url,
              time: new Date(aiMessage.time)
            })
          }
          
          this.setData({
            messages: messages,
            isStreaming: false
          })
          this.ensureScrollToBottom()
        } else {
          // 失败时更新占位气泡为错误提示
          const messages = [...(this.data.messages || [])]
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === 'ai' && messages[i].isStreaming) {
              messages[i].content = '发送失败'
              messages[i].isStreaming = false
              break
            }
          }
          this.setData({ messages, isStreaming: false })
          wx.showToast({ title: '发送失败', icon: 'error' })
        }
      },
      fail: () => {
        // 网络失败时更新占位气泡为错误提示
        const messages = [...(this.data.messages || [])]
        for (let i = messages.length - 1; i >= 0; i--) {
          if (messages[i].role === 'ai' && messages[i].isStreaming) {
            messages[i].content = '网络错误'
            messages[i].isStreaming = false
            break
          }
        }
        this.setData({ messages, isStreaming: false })
        wx.showToast({ title: '网络错误', icon: 'error' })
      }
    })
  },

  // 发送流式消息
  sendStreamMessage(message, imageUrls = []) {
    // 直接使用普通请求，但模拟流式效果
    this.sendNormalMessageWithStreamEffect(message, imageUrls)
  },

  // 回退到普通请求
  fallbackToNormalRequest(message) {
    console.log('回退到普通请求')
    
    // 不要移除流式传输状态，保持气泡存在
    // 直接使用普通请求，但模拟流式效果
    this.sendNormalMessageWithStreamEffect(message)
  },

  // 发送普通消息但模拟流式效果
  sendNormalMessageWithStreamEffect(message, imageUrls = []) {
    wx.request({
      url: app.globalData.baseUrl + '/api/chat/message',
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        character_id: this.data.characterId,
        message: message,
        image_urls: imageUrls
      },
      success: (res) => {
        if (res.statusCode === 200 && res.data.code === 200) {
          const userMessage = res.data.data.user_message
          const aiMessage = res.data.data.ai_message
          
          // 更新最后一条用户消息ID（从后向前查找最近的user）
          const messages = [...(this.data.messages || [])]
          let lastUserMessageIndex = -1
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === 'user') { lastUserMessageIndex = i; break }
          }
          if (lastUserMessageIndex !== -1) {
            messages[lastUserMessageIndex].id = userMessage.id
          }
          
          // 使用已有的AI占位气泡模拟流式
          const lastMessage = messages[messages.length - 1]
          if (lastMessage && lastMessage.isStreaming) {
            // 更新流式消息的真实ID
            lastMessage.id = aiMessage.id
            this.setData({ messages, isStreaming: true })
            // 模拟流式效果
            this.simulateStreamEffect(aiMessage.content, lastMessage.id)
          } else {
            // 兜底：如果没有找到占位气泡，创建新的
            const tempAiMessage = {
              id: aiMessage.id,
              role: 'ai',
              content: '',
              time: new Date(aiMessage.time),
              isStreaming: true
            }
            messages.push(tempAiMessage)
            this.setData({ messages, isStreaming: true })
            setTimeout(() => { this.scrollToBottom() }, 100)
            this.simulateStreamEffect(aiMessage.content, tempAiMessage.id)
          }
        } else {
          // 失败时更新占位气泡为错误提示
          const messages = [...(this.data.messages || [])]
          for (let i = messages.length - 1; i >= 0; i--) {
            if (messages[i].role === 'ai' && messages[i].isStreaming) {
              messages[i].content = '发送失败'
              messages[i].isStreaming = false
              break
            }
          }
          this.setData({ messages, isStreaming: false })
          wx.showToast({ title: '发送失败', icon: 'error' })
        }
      },
      fail: () => {
        // 网络失败时更新占位气泡为错误提示
        const messages = [...(this.data.messages || [])]
        for (let i = messages.length - 1; i >= 0; i--) {
          if (messages[i].role === 'ai' && messages[i].isStreaming) {
            messages[i].content = '网络错误'
            messages[i].isStreaming = false
            break
          }
        }
        this.setData({ messages, isStreaming: false })
        wx.showToast({ title: '网络错误', icon: 'error' })
      }
    })
  },

  // 模拟流式效果
  simulateStreamEffect(fullContent, messageId) {
    const messages = [...(this.data.messages || [])]
    const messageIndex = messages.findIndex(msg => msg.id === messageId)
    
    if (messageIndex === -1) return
    
    let currentIndex = 0
    const interval = setInterval(() => {
      if (currentIndex >= fullContent.length) {
        // 流式效果完成
        clearInterval(interval)
        messages[messageIndex].isStreaming = false
        
        this.setData({
          messages: messages,
          isStreaming: false
        })
        
        this.scrollToBottom()
        console.log('模拟流式效果完成')
        return
      }
      
      // 添加下一个字符
      messages[messageIndex].content += fullContent[currentIndex]
      currentIndex++
      
      this.setData({
        messages: messages
      })
      
      this.scrollToBottom()
    }, 50) // 每50ms添加一个字符
  },

  // 处理流式数据块
  handleStreamChunk(res) {
    console.log('处理流式数据块:', res)
    
    try {
      const data = res.data
      console.log('原始数据:', data)
      
      if (data && typeof data === 'string') {
        // 处理SSE格式数据
        if (data.startsWith('data: ')) {
          const content = data.substring(6) // 移除 'data: ' 前缀
          console.log('解析后的内容:', content)
          
          if (content === '[START]') {
            // 流式传输开始
            console.log('流式传输开始')
          } else if (content === '[END]') {
            // 流式传输结束
            console.log('流式传输结束')
            this.finishStreaming()
          } else if (content.startsWith('[ERROR]')) {
            // 流式传输错误
            const errorMsg = content.substring(8)
            console.error('流式传输错误:', errorMsg)
            this.handleStreamError(errorMsg)
          } else {
            // 正常内容块
            console.log('追加内容:', content)
            this.appendStreamContent(content)
          }
        } else {
          // 直接处理内容（非SSE格式）
          console.log('直接处理内容:', data)
          this.appendStreamContent(data)
        }
      } else {
        console.log('数据格式异常:', typeof data, data)
      }
    } catch (error) {
      console.error('处理流式数据块失败:', error)
      this.handleStreamError()
    }
  },

  // 追加流式内容
  appendStreamContent(content) {
    console.log('追加流式内容:', content)
    
    const messages = [...(this.data.messages || [])]
    const lastMessage = messages[messages.length - 1]
    
    if (lastMessage && lastMessage.isStreaming) {
      lastMessage.content += content
      
      this.setData({
        messages: messages
      })
      
      // 实时滚动到底部
      this.scrollToBottom()
      
      console.log('内容已追加，当前长度:', lastMessage.content.length)
    } else {
      console.log('没有找到正在流式传输的消息')
    }
  },

  // 完成流式传输
  finishStreaming() {
    console.log('完成流式传输')
    
    const messages = [...(this.data.messages || [])]
    const lastMessage = messages[messages.length - 1]
    
    if (lastMessage && lastMessage.isStreaming) {
      lastMessage.isStreaming = false
      
      this.setData({
        messages: messages,
        isStreaming: false,
        currentStreamingMessage: null
      })
      
      // 最终滚动到底部
      this.scrollToBottom()
      
      console.log('流式传输完成，最终内容:', lastMessage.content)
    } else {
      console.log('没有找到正在流式传输的消息')
    }
  },

  // 处理流式传输错误
  handleStreamError(errorMsg = '流式传输失败') {
    console.error('处理流式传输错误:', errorMsg)
    
    const messages = [...(this.data.messages || [])]
    const lastMessage = messages[messages.length - 1]
    
    if (lastMessage && lastMessage.isStreaming) {
      lastMessage.content = errorMsg
      lastMessage.isStreaming = false
      
      this.setData({
        messages: messages,
        isStreaming: false,
        currentStreamingMessage: null
      })
      
      wx.showToast({
        title: errorMsg,
        icon: 'error'
      })
    }
  },

  // 滚动到底部
  scrollToBottom() {
    setTimeout(() => {
      const messages = this.data.messages || []
      if (messages.length > 0) {
        const lastMessage = messages[messages.length - 1]
        this.setData({
          scrollToMessage: `msg-${lastMessage.id}`
        })
      }
    }, 50) // 减少延迟，确保及时滚动
  },

  // 强制滚动到底部（用于页面加载时）
  forceScrollToBottom() {
    setTimeout(() => {
      const messages = this.data.messages || []
      if (messages.length > 0) {
        const lastMessage = messages[messages.length - 1]
        this.setData({
          scrollToMessage: `msg-${lastMessage.id}`
        })
      }
    }, 100) // 减少延迟
  },

  // 确保滚动到底部（多次尝试）
  ensureScrollToBottom() {
    const messages = this.data.messages || []
    if (messages.length > 0) {
      const lastMessage = messages[messages.length - 1]
      this.setData({
        scrollToMessage: `msg-${lastMessage.id}`
      })
      
      // 多次尝试确保滚动成功
      setTimeout(() => {
        this.setData({
          scrollToMessage: `msg-${lastMessage.id}`
        })
      }, 50)
      
      setTimeout(() => {
        this.setData({
          scrollToMessage: `msg-${lastMessage.id}`
        })
      }, 150)
    }
  },

  // 页面显示
  onShow() {
    // 检查本地是否有通话时长消息
    const lastMsg = wx.getStorageSync('lastCallDurationMsg');
    const lastTime = wx.getStorageSync('lastCallDurationTime');
    const lastDialogId = wx.getStorageSync('lastCallDialogId');
    if (lastMsg && lastDialogId == this.data.dialogId) {
      // 检查当前消息列表中是否已经有通话时长消息
      const hasCallDurationMessage = this.data.messages && this.data.messages.some(msg => 
        msg.content && msg.content.includes('通话时长:')
      );
      
      if (!hasCallDurationMessage) {
      // 插入到消息列表
      const msg = {
          id: 'temp_call_duration_' + Date.now(), // 临时ID，标识为通话时长消息
        content: lastMsg,
        is_voice: 'no',
          role: 'user', // 改为'user'，因为这是用户发送的消息
        time: lastTime || new Date().toISOString()
      };
        
        // 添加到现有消息列表，而不是覆盖
        const currentMessages = this.data.messages || [];
        const updatedMessages = [...currentMessages, msg];
        
        this.setData({ 
          messages: updatedMessages
        });
        
        // 滚动到底部显示通话时长消息
        setTimeout(() => {
          this.ensureScrollToBottom();
        }, 100);
      }
      
      // 清除本地存储
      wx.removeStorageSync('lastCallDurationMsg');
      wx.removeStorageSync('lastCallDurationTime');
      wx.removeStorageSync('lastCallDialogId');
    }
  },

  // 页面卸载
  onUnload() {
    // 清理流式传输状态
    if (this.data.isStreaming) {
      this.setData({
        isStreaming: false,
        currentStreamingMessage: null
      })
    }
  },

  // 滚动事件
  onScroll(e) {
    // 可以在这里添加滚动相关的处理逻辑
  },

  // 滚动到顶部
  onScrollToUpper() {
    // 可以在这里添加加载历史消息的逻辑
  },


  // 消息长按事件
  onMessageLongPress(e) {
    const message = e.currentTarget.dataset.message
    const role = e.currentTarget.dataset.role
    const messageId = e.currentTarget.dataset.messageId
    
    console.log('长按消息 - 消息ID:', messageId, '角色:', role, '内容:', message)
    
    let itemList = ['复制']
    
    // 如果是AI消息，添加"复制并发送"选项
    if (role === 'ai') {
      itemList.push('复制并发送')
    }
    
    // 添加删除选项
    itemList.push('删除')
    
    wx.showActionSheet({
      itemList: itemList,
      success: (res) => {
        const index = res.tapIndex
        if (index === 0) {
          // 复制
          this.copyMessage(message)
        } else if (index === 1 && role === 'ai') {
          // 复制并发送（仅AI消息）
          this.copyAndSendMessage(message)
        } else if ((index === 1 && role === 'user') || (index === 2 && role === 'ai')) {
          // 删除消息
          console.log('用户选择删除消息 - 消息ID:', messageId)
          this.deleteMessage(messageId)
        }
      }
    })
  },

  // 复制消息
  copyMessage(message) {
    if (!message || message.trim() === '') {
      wx.showToast({
        title: '消息内容为空',
        icon: 'none',
        duration: 1500
      })
      return
    }
    
          wx.setClipboardData({
            data: message,
            success: () => {
              wx.showToast({
                title: '已复制到剪贴板',
          icon: 'success',
          duration: 1500
        })
      },
      fail: () => {
        wx.showToast({
          title: '复制失败',
          icon: 'error',
          duration: 1500
              })
            }
          })
  },

  // 复制并发送消息
  copyAndSendMessage(message) {
    if (!message || message.trim() === '') {
      wx.showToast({
        title: '消息内容为空',
        icon: 'none',
        duration: 1500
      })
      return
    }
    
    // 先复制到剪贴板
    wx.setClipboardData({
      data: message,
      success: () => {
        // 然后设置为输入框内容
        this.setData({
          inputMessage: message
        })
        wx.showToast({
          title: '已复制到输入框',
          icon: 'success',
          duration: 1500
        })
      },
      fail: () => {
        wx.showToast({
          title: '复制失败',
          icon: 'error',
          duration: 1500
        })
      }
    })
  },

  // 长按输入框
  onInputLongPress(e) {
    // 显示操作菜单
    wx.showActionSheet({
      itemList: ['粘贴', '清空'],
      success: (res) => {
        switch (res.tapIndex) {
          case 0: // 粘贴
            this.pasteFromClipboard()
            break
          case 1: // 清空
            this.clearInput()
            break
        }
      }
    })
  },

  // 粘贴功能
  pasteFromClipboard() {
    wx.getClipboardData({
      success: (res) => {
        if (res.data) {
          const currentText = this.data.inputMessage || ''
          const newText = currentText + res.data
          this.setData({
            inputMessage: newText
          })
          wx.showToast({
            title: '已粘贴',
            icon: 'success',
            duration: 1000
          })
        } else {
          wx.showToast({
            title: '剪贴板为空',
            icon: 'none',
            duration: 1500
          })
        }
      },
      fail: () => {
        wx.showToast({
          title: '粘贴失败',
          icon: 'error',
          duration: 1500
        })
      }
    })
  },

  // 清空输入框
  clearInput() {
    this.setData({
      inputMessage: ''
    })
    wx.showToast({
      title: '已清空',
      icon: 'success',
      duration: 1000
    })
  },

  // 新增：选择图片
  selectImages() {
    const remainingCount = 5 - this.data.selectedImages.length
    if (remainingCount <= 0) {
      wx.showToast({
        title: '最多只能选择5张图片',
        icon: 'none',
        duration: 2000
      })
      return
    }

    wx.chooseMedia({
      count: remainingCount,
      mediaType: ['image'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        // 若用户未选择任何文件，静默返回
        if (!res || !res.tempFiles || res.tempFiles.length === 0) return
        const tempFilePaths = res.tempFiles.map(file => file.tempFilePath)
        this.setData({
          selectedImages: [...this.data.selectedImages, ...tempFilePaths]
        })
      },
      fail: (err) => {
        // 用户取消选择时静默处理；其他错误再提示
        const msg = (err && (err.errMsg || '')).toLowerCase()
        const isCancel = msg.includes('cancel')
        if (isCancel) return
        console.error('选择图片失败:', err)
        wx.showToast({
          title: '选择图片失败',
          icon: 'error',
          duration: 2000
        })
      }
    })
  },

  // 新增：移除图片
  removeImage(e) {
    const index = e.currentTarget.dataset.index
    const selectedImages = [...this.data.selectedImages]
    selectedImages.splice(index, 1)
    this.setData({
      selectedImages
    })
  },

  // 新增：上传图片到服务器
  uploadImages() {
    return new Promise((resolve, reject) => {
      if (this.data.selectedImages.length === 0) {
        resolve([])
        return
      }

      wx.showLoading({
        title: '上传图片中...'
      })

      const uploadTasks = this.data.selectedImages.map(imagePath => {
        return new Promise((resolveUpload, rejectUpload) => {
          wx.uploadFile({
            url: app.globalData.baseUrl + '/api/chat/upload-images',
            filePath: imagePath,
            name: 'images',
            header: {
              'Authorization': 'Bearer ' + app.globalData.token
            },
            success: (res) => {
              try {
                const result = JSON.parse(res.data)
                if (result.code === 200) {
                  resolveUpload(result.data.image_urls[0])
                } else {
                  rejectUpload(new Error(result.message || '上传失败'))
                }
              } catch (error) {
                rejectUpload(new Error('解析响应失败'))
              }
            },
            fail: (error) => {
              rejectUpload(error)
            }
          })
        })
      })

      Promise.all(uploadTasks)
        .then(imageUrls => {
          wx.hideLoading()
          resolve(imageUrls)
        })
        .catch(error => {
          wx.hideLoading()
          console.error('上传图片失败:', error)
          wx.showToast({
            title: '图片上传失败',
            icon: 'error',
            duration: 2000
          })
          reject(error)
        })
    })
  },

  // 新增：图片预览
  previewImage(e) {
    const current = e.currentTarget.dataset.current
    const urls = e.currentTarget.dataset.urls
    
    wx.previewImage({
      current: current,
      urls: urls
    })
  },

  // 新增：处理消息中的图片URL
  processMessageImages(message) {
    if (message.picture_url) {
      const urls = message.picture_url.split(',').filter(url => url.trim() !== '')
      message.images = urls
    } else {
      message.images = []
    }
    return message
  },

  // 新增：删除消息
  deleteMessage(messageId) {
    wx.showModal({
      title: '确认删除',
      content: '确定要删除这条消息吗？',
      confirmText: '删除',
      confirmColor: '#ff4757',
      success: (res) => {
        if (res.confirm) {
          this.performDeleteMessage(messageId)
        }
      }
    })
  },

  // 执行删除消息
  performDeleteMessage(messageId) {
    console.log('开始删除消息 - 消息ID:', messageId)
    
    wx.showLoading({
      title: '删除中...'
    })

    wx.request({
      url: app.globalData.baseUrl + '/api/chat/delete-message',
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token,
        'Content-Type': 'application/json'
      },
      data: {
        message_id: messageId
      },
      success: (res) => {
        wx.hideLoading()
        console.log('删除消息响应:', res)
        if (res.statusCode === 200 && res.data.code === 200) {
          wx.showToast({
            title: '删除成功',
            icon: 'success'
          })
          // 重新加载对话以更新显示
          this.loadDialog()
        } else {
          console.error('删除失败:', res.data)
          wx.showToast({
            title: res.data.message || '删除失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('删除消息失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      }
    })
  }
}) 