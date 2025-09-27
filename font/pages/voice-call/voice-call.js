const app = getApp()


Page({
  data: {
    characterId: null,
    characterName: '',
    characterAvatar: '',
    isCalling: false,
    callDuration: 0,
    isRecording: false,
    isPlaying: false,
    isThinking: false, // 新增：AI思考状态
    showMessages: false, // 新增：是否显示消息区域
    isProcessingAI: false, // 新增：是否正在处理AI回复
    voiceQueue: [], // 新增：语音播放队列
    currentSequence: 0, // 新增：当前播放序号
    totalSequences: 0, // 新增：总序号数量
    ws: null,
    recorderManager: null,
    innerAudioContext: null,
    webAudioContext: null, // 新增：WebAudioContext
    callTimer: null,
    dialogId: null,
    messages: [], // 新增：消息列表
    scrollToView: '', // 新增：滚动到指定消息
    messageIdCounter: 0, // 新增：消息ID计数器
    currentAudioContext: null, // 新增：当前播放的音频上下文
    isPausingRecording: false, // 新增：是否正在暂停录音
    playedUrls: new Set(), // 新增：记录已播放的URL，避免重复播放
    shouldStartPlaying: false, // 新增：播放标志，控制是否应该开始播放
    playbackStarted: false, // 新增：是否已经开始播放的标志
    silentFrameTimer: null, // 新增：静音帧定时器
    silentFrameBuffer: null, // 新增：静音帧缓存
    hasReceivedFinal: false, // 新增：是否已接收到final结果，防止重复处理
    
    // 新增：双缓冲区相关状态
    smallBuffer: [], // 小缓冲区：存储PCM数据片段
    largeBuffer: [], // 大缓冲区：存储合并后的音频文件
    smallBufferSize: 0, // 小缓冲区当前大小
    maxSmallBufferSize: 100000, // 小缓冲区最大大小（字节）
    isPlayingFromLargeBuffer: false, // 是否正在从大缓冲区播放
    currentPlayingIndex: 0, // 当前播放的音频索引
    audioContexts: [], // 存储所有音频上下文
    
    // 新增：定时器相关状态
    smallBufferTimer: null, // 小缓冲区定时器
    smallBufferTimeout: 2000, // 小缓冲区超时时间（毫秒）
    lastSmallBufferUpdate: 0, // 最后小缓冲区更新时间戳
    
    // 新增：对话轮次管理
    currentDialogRound: 0, // 当前对话轮次
    isNewDialogRound: false, // 是否是新的一轮对话
    
    // 新增：打断状态管理
    isInterrupted: false, // 是否已被打断
    isTopAvatarTall: false, // 新增：顶部/AI头像是否为长图
    userInfo: null // 新增：用户信息，用于显示用户头像
  },

  onLoad(options) {
    console.log('语音通话页面加载，参数:', options)
    
    this.setData({
      characterId: parseInt(options.characterId),
      characterName: options.characterName,
      characterAvatar: options.characterAvatar,
      dialogId: options.dialogId ? parseInt(options.dialogId) : 0
    })
    
    // 设置页面配置，确保可以正常返回
    wx.setNavigationBarTitle({
      title: `与${options.characterName}通话`
    })
    
    // 设置角色信息
    this.setData({
      characterId: Number(options.characterId),
      characterName: options.characterName || '',
      characterAvatar: options.characterAvatar || ''
    })
    // 注入用户信息
    this.setData({ userInfo: app.globalData.userInfo || null })
    
    // 初始化录音管理器
    this.initRecorderManager()
    
    // 初始化音频播放器
    this.initAudioPlayer()
    
    // 初始化WebAudioContext
    this.initWebAudioContext()
    
    // 直接开始通话
    this.startCall()
    this.lastRecordFilePath = null // 新增：用于存储对话ID
    this.audioFrames = [] // 新增：用于存储音频帧
    // 初始化静音帧缓存
    this.data.silentFrameBuffer = this.generateSilentFrame();
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

  // 新增：初始化WebAudioContext
  initWebAudioContext() {
    try {
      this.setData({
        webAudioContext: wx.createWebAudioContext()
      })
      console.log('WebAudioContext初始化成功')
    } catch (error) {
      console.error('WebAudioContext初始化失败:', error)
    }
  },

  // 自定义返回按钮点击事件
  onCustomBackTap() {
    if (this.data.isCalling) {
      // 如果正在通话中，提示用户是否结束通话
      wx.showModal({
        title: '结束通话',
        content: '确定要结束当前通话吗？',
        confirmText: '结束通话',
        confirmColor: '#ff4757',
        cancelText: '继续通话',
        success: (res) => {
          if (res.confirm) {
            // 用户确认结束通话
            this.endCall()
          }
        }
      })
    } else {
      // 如果不在通话中，直接返回
      wx.navigateBack()
    }
  },

  onUnload() {
    console.log('页面卸载，结束通话')
    
    // 停止静音帧发送
    this.stopSilentFrames();
    
    // 停止所有音频播放
    this.stopAllAudio()
    
    // 销毁WebAudioContext
    if (this.data.webAudioContext) {
      try {
        this.data.webAudioContext.close()
        this.setData({ webAudioContext: null })
        console.log('WebAudioContext已销毁')
      } catch (error) {
        console.error('销毁WebAudioContext失败:', error)
      }
    }
    
    // 清理双缓冲区
    this.clearAudioBuffers()
    
    // 确保通话状态正确设置
    this.setData({ isCalling: false })
    this.endCall()
  },

  // 初始化录音管理器
  initRecorderManager() {
    this.data.recorderManager = wx.getRecorderManager()
    
    // 录音开始事件
    this.data.recorderManager.onStart(() => {
      console.log('录音开始')
      this.setData({ isRecording: true })
      this.audioFrames = [] // 清空音频帧数组
    })
    
    // 录音结束事件
    this.data.recorderManager.onStop((res) => {
      console.log('录音结束', res)
      this.setData({ isRecording: false })
      // 保存录音文件路径（仅用于调试）
      this.lastRecordFilePath = res.tempFilePath
    })
    
    // 录音错误事件
    this.data.recorderManager.onError((err) => {
      console.error('录音错误', err)
      this.setData({ isRecording: false })
      wx.showToast({
        title: '录音失败',
        icon: 'error'
      })
    })
    
    // 录音帧数据事件
    this.data.recorderManager.onFrameRecorded((res) => {
      const { frameBuffer, isLastFrame } = res
      
      // 转换为Uint8Array
      const uint8Array = new Uint8Array(frameBuffer)

      // 通过WebSocket发送音频数据
      // 新增：检查是否已接收到final结果，如果是则停止发送音频帧
      if (this.data.ws && this.data.isCalling && !this.data.hasReceivedFinal) {
        const now = new Date().toISOString()
        this.data.ws.send({
          data: frameBuffer,
          success: () => {},
          fail: (err) => {
            console.error('❌ PCM音频帧发送失败:', err)
          }
        })
      } else if (this.data.hasReceivedFinal) {
        console.log('已接收到final结果，停止发送音频帧数据')
      } else {
        console.warn('WebSocket连接不可用或通话已结束，跳过音频帧发送')
      }
    })
  },

  // 初始化音频播放器
  initAudioPlayer() {
    this.data.innerAudioContext = wx.createInnerAudioContext()
    
    this.data.innerAudioContext.onPlay(() => {
      console.log('开始播放')
      this.setData({ isPlaying: true })
    })
    
    this.data.innerAudioContext.onEnded(() => {
      console.log('播放结束')
      this.setData({ isPlaying: false })
    })
    
    this.data.innerAudioContext.onError((err) => {
      console.error('播放错误', err)
      this.setData({ isPlaying: false })
      wx.showToast({
        title: '播放失败',
        icon: 'error'
      })
    })
  },

  // 开始通话
  startCall() {
    if (this.data.isCalling) {
      return
    }
    
    this.setData({ isCalling: true })
    
    // 建立WebSocket连接
    this.connectWebSocket()
    
    // 开始计时
    this.startCallTimer()
    
    // 开始录音
    this.startRecording()
  },

  // 结束通话
  endCall() {
    if (!this.data.isCalling) return;

    // 停止静音帧发送
    this.stopSilentFrames();

    // 1. 停止所有音频播放
    this.stopAllAudio();

    // 2. 计算本地通话时长
    const duration = this.data.callDuration || 0;
    const min = Math.floor(duration / 60);
    const sec = duration % 60;
    const callDurationText = `通话时长: ${min}分${sec}秒`;

    // 3. 存储到本地，供聊天页展示
    wx.setStorageSync('lastCallDurationMsg', callDurationText);
    wx.setStorageSync('lastCallDurationTime', new Date().toISOString());
    wx.setStorageSync('lastCallDialogId', this.data.dialogId);

    // 4. 发送通话时长到后端
    wx.request({
      url: app.globalData.baseUrl + '/api/messages', // 假设后端消息接口
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        dialog_id: this.data.dialogId,
        character_id: this.data.characterId, // 新增：传递角色ID
        content: callDurationText,
        is_voice: 'no',
        role: 'user', // 这里改为'user'
        time: new Date().toISOString()
      },
      success: res => {
        console.log('通话时长已发送到后端', res);
      }
    });

    // 5. 清理和返回
    this.setData({ 
      isCalling: false,
      hasReceivedFinal: false // 新增：重置final结果标志
    });
    this.stopRecording();
    this.stopCallTimer();
    
    // 额外的音频清理
    if (this.data.innerAudioContext) {
      try {
        this.data.innerAudioContext.stop();
        this.data.innerAudioContext.destroy();
        this.data.innerAudioContext = null;
      } catch (error) {
        console.log('清理音频上下文失败:', error);
      }
    }
    
    this.setData({ isPlaying: false });
    this.closeWebSocket();

    // 6. 显示模态框并返回
    wx.showModal({
      title: '通话结束',
      content: callDurationText,
      showCancel: false,
      confirmText: '确定',
      success: () => {
        // 返回上一页并触发重新加载
        wx.navigateBack({
          success: () => {
            // 延迟一下确保页面已经返回
            setTimeout(() => {
              // 获取当前页面栈
              const pages = getCurrentPages();
              const chatPage = pages[pages.length - 1];
              // 如果当前页面是聊天页面，重新加载消息
              if (chatPage && chatPage.loadDialog) {
                chatPage.loadDialog();
                // 延迟滚动到底部，确保消息加载完成
                setTimeout(() => {
                  if (chatPage.ensureScrollToBottom) {
                    chatPage.ensureScrollToBottom();
                  }
                }, 500);
              }
            }, 100);
          }
        });
      }
    });
  },

  // 建立WebSocket连接
  connectWebSocket() {
    // 使用app.js中配置的wsUrl，通过URL参数传递token
    const wsURL = app.globalData.wsUrl + '?token=' + encodeURIComponent(app.globalData.token || '')
    
    console.log('连接WebSocket:', wsURL)
    
    // 设置连接超时
    const connectionTimeout = setTimeout(() => {
      console.error('WebSocket连接超时')
      wx.showToast({
        title: '连接超时',
        icon: 'error'
      })
      this.endCall()
    }, 15000) // 15秒超时
    
    this.data.ws = wx.connectSocket({
      url: wsURL,
      success: () => {
        console.log('WebSocket连接成功')
      },
      fail: (err) => {
        console.error('WebSocket连接失败', err)
        clearTimeout(connectionTimeout)
        wx.showToast({
          title: '连接失败',
          icon: 'error'
        })
        this.endCall()
      }
    })
    
    // 监听连接打开
    this.data.ws.onOpen(() => {
      console.log('WebSocket连接已打开')
      clearTimeout(connectionTimeout) // 清除超时定时器
      
      // 发送开始通话消息
      this.data.ws.send({
        data: JSON.stringify({
          type: 'start_call',
          character_id: this.data.characterId,
          dialog_id: this.data.dialogId // 使用this.data.dialogId
        }),
        success: () => {
          console.log('发送开始通话消息成功')
        },
        fail: (err) => {
          console.error('发送开始通话消息失败', err)
        }
      })
    })
    
    // 监听消息
    this.data.ws.onMessage((res) => {
      this.handleWebSocketMessage(res.data)
    })
    
    // 监听错误
    this.data.ws.onError((err) => {
      console.error('WebSocket错误', err)
      clearTimeout(connectionTimeout)
      wx.showToast({
        title: '连接错误',
        icon: 'error'
      })
    })
    
    // 监听关闭
    this.data.ws.onClose(() => {
      console.log('WebSocket连接已关闭')
      clearTimeout(connectionTimeout)
    })
  },

  // 关闭WebSocket连接
  closeWebSocket() {
    if (this.data.ws) {
      console.log('准备关闭WebSocket连接')
      
      try {
        // 发送结束通话消息
        this.data.ws.send({
          data: JSON.stringify({
            type: 'end_call',
            dialog_id: this.data.dialogId // 使用this.data.dialogId
          }),
          success: () => {
            console.log('结束通话消息发送成功')
          },
          fail: (err) => {
            console.error('发送结束通话消息失败:', err)
          }
        })
        
        // 关闭连接
        this.data.ws.close({
          success: () => {
            console.log('WebSocket连接关闭成功')
          },
          fail: (err) => {
            console.error('WebSocket连接关闭失败:', err)
          }
        })
        
        this.data.ws = null
        console.log('WebSocket连接已清理')
      } catch (error) {
        console.error('关闭WebSocket连接时出错:', error)
        this.data.ws = null
      }
    } else {
      console.log('WebSocket连接不存在，无需关闭')
    }
  },

  // 处理WebSocket消息
  handleWebSocketMessage(data) {
    try {
      const message = JSON.parse(data)
      console.log('收到WebSocket消息', message)
      
      switch (message.type) {
        case 'call_started':
          console.log('通话已开始')
          wx.showToast({
            title: '通话已开始',
            icon: 'success'
          })
          // 新增：如果后端返回dialog_id，赋值到this.data.dialogId
          if (message.data && message.data.dialog_id) {
            this.setData({ dialogId: message.data.dialog_id })
          }
          break
          
        case 'call_ended':
          console.log('通话已结束')
          // 注意：不再调用this.endCall()，因为前端已经调用了closeWebSocket()
          // 这里只是确认后端也结束了通话
          break
          
        case 'call_duration':
          // 收到通话时长消息
          console.log('通话时长:', message.data)
          
          // 只显示Toast提示，不重复返回
          wx.showToast({
            title: message.data,
            icon: 'none',
            duration: 2000
          })
          break
          
        case 'recognition_result':
          // 显示语音识别结果（支持流式）
          console.log('语音识别结果:', message.data)
          
          // 检查是否为对象格式（包含is_final标识）
          if (typeof message.data === 'object' && message.data.text) {
            const { text, is_final } = message.data
            
            if (is_final) {
              // 防止重复处理
              if (this.data.isProcessingAI || this.data.hasReceivedFinal) {
                console.log('正在处理AI回复或已接收到final结果，跳过重复请求')
                break
              }
              
              // 接收到final结果，立即停止录音并设置AI处理状态
              console.log('接收到final结果，开始AI处理流程')
              
              // 立即停止录音，防止继续发送音频帧
              if (this.data.isRecording) {
                this.stopRecording()
              }
              
              this.setData({ 
                isProcessingAI: true,
                isPausingRecording: true, // 立即暂停录音
                isThinking: true,
                hasReceivedFinal: true // 新增：标记已接收到final结果
              })
              
              // 最终结果：添加到消息列表
              this.addUserMessage(text)
            } else {
              // 中间结果：显示流式效果
              // 新增：如果已接收到final结果，则忽略中间结果
              if (!this.data.hasReceivedFinal) {
              this.showStreamingResult(text)
              }
            }
          } else {
            // 兼容旧格式：直接显示文本
            if (!this.data.isProcessingAI && !this.data.hasReceivedFinal) {
              this.setData({ 
                isProcessingAI: true,
                hasReceivedFinal: true // 新增：标记已接收到final结果
              })
              this.addUserMessage(message.data)
              this.setData({ isThinking: true })
            }
          }
          break
          
        case 'ai_response':
          // 显示AI文本回复
          console.log('AI回复:', message.data)
          this.setData({ 
            isThinking: false,
            isProcessingAI: false,  // 重置处理状态
            hasReceivedFinal: false // 新增：重置final结果标志，允许用户继续说话
          })
          this.addAIMessage(message.data)
          break
          
        case 'stream_audio':
          // 处理流式音频数据
          console.log('收到流式音频数据:', message.data)
          this.handleStreamAudio(message.data)
          break
          
        case 'interrupt_confirmed':
          // 后端确认打断信号
          console.log('后端确认打断信号:', message.data)
          // 清空所有缓冲区，准备接收新的音频数据
          this.clearAudioBuffers()
          this.setData({
            isPlaying: false,
            currentSequence: 0,
            isPlayingFromLargeBuffer: false,
            currentPlayingIndex: 0,
            isInterrupted: false // 重置打断状态
          })
          break
          
        case 'error':
          console.error('服务器错误:', message.message)
          this.setData({ 
            isThinking: false,
            isProcessingAI: false,  // 重置处理状态
            hasReceivedFinal: false, // 新增：重置final结果标志
            isInterrupted: false // 重置打断状态
          })
          wx.showToast({
            title: message.message || '服务器错误',
            icon: 'error'
          })
          break
          
        default:
          console.log('未知消息类型:', message.type)
      }
    } catch (error) {
      console.error('解析WebSocket消息失败:', error)
    }
  },

  // 新增：处理流式音频数据
  handleStreamAudio(audioData) {
    console.log('处理流式音频，序号:', audioData.sequence)
    
    // 检查是否已被打断，如果是则忽略此音频数据
    if (this.data.isInterrupted) {
      console.log('音频播放已被打断，忽略此音频数据')
      return
    }
    
    // 检测是否是新的一轮对话
    this.detectNewDialogRound(audioData.sequence)
    
    // 暂停录音
    this.setData({ 
      isPlaying: true,
      isPausingRecording: true 
    })
    this.pauseCall()
    
    // 将base64音频数据转换为ArrayBuffer
    const binaryData = wx.base64ToArrayBuffer(audioData.audio_data)
    console.log('音频数据大小:', binaryData.byteLength, 'bytes')
    
    // 添加到小缓冲区
    this.addToSmallBuffer(binaryData, audioData.sequence)
  },

  // 新增：检测新对话轮次
  detectNewDialogRound(sequence) {
    // 如果序号为1，说明是新的一轮对话
    if (sequence === 1) {
      console.log('检测到新对话轮次，清空缓冲区')
      this.startNewDialogRound()
    }
  },

  // 新增：开始新对话轮次
  startNewDialogRound() {
    // 停止当前播放
    this.stopCurrentPlayback()
    
    // 清空缓冲区
    this.clearAudioBuffers()
    
    // 更新对话轮次
    this.data.currentDialogRound++
    this.data.isNewDialogRound = true
    
    console.log(`开始新对话轮次: ${this.data.currentDialogRound}`)
    
    // 重置播放状态
    this.setData({
      isPlayingFromLargeBuffer: false,
      currentPlayingIndex: 0
    })
    
    // 清理旧对话轮次的音频（如果有的话）
    this.clearOldDialogRounds()
  },

  // 新增：停止当前播放
  stopCurrentPlayback() {
    // 停止所有音频上下文
    this.data.audioContexts.forEach(context => {
      try {
        if (context && typeof context.stop === 'function') {
          context.stop()
        }
      } catch (error) {
        console.log('停止音频上下文失败:', error)
      }
    })
    this.data.audioContexts = []
    
    console.log('已停止当前播放')
  },

  // 新增：添加到小缓冲区
  addToSmallBuffer(audioData, sequence) {
    try {
      // 将ArrayBuffer转换为Int16Array
      const pcmData = new Int16Array(audioData)
      
      // 添加到小缓冲区，标记对话轮次
      this.data.smallBuffer.push({
        data: pcmData,
        sequence: sequence,
        size: pcmData.byteLength,
        dialogRound: this.data.currentDialogRound // 添加对话轮次标记
      })
      
      this.data.smallBufferSize += pcmData.byteLength
      
      console.log(`小缓冲区添加音频片段 ${sequence}，对话轮次: ${this.data.currentDialogRound}，大小: ${pcmData.byteLength} bytes，总大小: ${this.data.smallBufferSize} bytes`)
      
      // 更新最后更新时间戳
      this.data.lastSmallBufferUpdate = Date.now()
      
      // 重置定时器
      this.resetSmallBufferTimer()
      
      // 检查是否需要合并（基于大小）
      if (this.data.smallBufferSize >= this.data.maxSmallBufferSize) {
        this.mergeSmallBuffer()
      }
      
      // 开始播放（如果还没有开始）
      if (!this.data.isPlayingFromLargeBuffer) {
        this.startPlayingFromLargeBuffer()
      }
      
    } catch (error) {
      console.error('添加到小缓冲区失败:', error)
      this.resumeRecordingAfterPlayback()
    }
  },

  // 新增：重置小缓冲区定时器
  resetSmallBufferTimer() {
    // 清除现有定时器
    if (this.data.smallBufferTimer) {
      clearTimeout(this.data.smallBufferTimer)
      this.data.smallBufferTimer = null
    }
    
    // 设置新的定时器
    this.data.smallBufferTimer = setTimeout(() => {
      this.checkSmallBufferTimeout()
    }, this.data.smallBufferTimeout)
  },

  // 新增：清理旧对话轮次的音频
  clearOldDialogRounds() {
    // 清理大缓冲区中非当前轮次的音频
    this.data.largeBuffer = this.data.largeBuffer.filter(item => 
      item.dialogRound === this.data.currentDialogRound
    )
    
    console.log(`清理旧对话轮次音频，保留当前轮次: ${this.data.currentDialogRound}，剩余音频数量: ${this.data.largeBuffer.length}`)
  },

  // 新增：手动合并小缓冲区
  forceMergeSmallBuffer() {
    if (this.data.smallBuffer.length > 0) {
      console.log('手动合并小缓冲区')
      this.mergeSmallBuffer()
    }
  },

  // 新增：检查小缓冲区超时
  checkSmallBufferTimeout() {
    const now = Date.now()
    const timeSinceLastUpdate = now - this.data.lastSmallBufferUpdate
    
    console.log(`检查小缓冲区超时，距离上次更新: ${timeSinceLastUpdate}ms`)
    
    // 如果超过超时时间且小缓冲区有数据，则合并
    if (timeSinceLastUpdate >= this.data.smallBufferTimeout && this.data.smallBuffer.length > 0) {
      console.log('小缓冲区超时，强制合并')
      this.mergeSmallBuffer()
    }
  },

  // 新增：合并小缓冲区
  mergeSmallBuffer() {
    try {
      console.log('开始合并小缓冲区，片段数量:', this.data.smallBuffer.length)
      
      // 计算总长度
      let totalLength = 0
      for (const item of this.data.smallBuffer) {
        totalLength += item.data.length
      }
      
      // 创建合并后的PCM数据
      const mergedPcmData = new Int16Array(totalLength)
      let offset = 0
      
      for (const item of this.data.smallBuffer) {
        mergedPcmData.set(item.data, offset)
        offset += item.data.length
      }
      
      // 创建AudioBuffer
      const audioBuffer = this.createAudioBufferFromPCM(mergedPcmData, 22050, 1)
      
      if (audioBuffer && audioBuffer.duration > 0) {
        // 添加到大缓冲区，标记对话轮次
        this.data.largeBuffer.push({
          audioBuffer: audioBuffer,
          sequences: this.data.smallBuffer.map(item => item.sequence),
          duration: audioBuffer.duration,
          dialogRound: this.data.currentDialogRound // 添加对话轮次标记
        })
        
        console.log(`合并完成，对话轮次: ${this.data.currentDialogRound}，音频时长: ${audioBuffer.duration.toFixed(3)}秒，大缓冲区数量: ${this.data.largeBuffer.length}`)
      } else {
        console.error('创建合并音频缓冲区失败')
      }
      
      // 清空小缓冲区
      this.data.smallBuffer = []
      this.data.smallBufferSize = 0
      
      // 重置定时器
      this.resetSmallBufferTimer()
      
    } catch (error) {
      console.error('合并小缓冲区失败:', error)
    }
  },

  // 新增：从大缓冲区开始播放
  startPlayingFromLargeBuffer() {
    if (this.data.largeBuffer.length === 0) {
      console.log('大缓冲区为空，等待更多音频数据')
      return
    }
    
    this.setData({ isPlayingFromLargeBuffer: true })
    console.log('开始从大缓冲区播放，音频数量:', this.data.largeBuffer.length)
    
    this.playNextFromLargeBuffer()
  },

  // 新增：播放大缓冲区中的下一个音频
  playNextFromLargeBuffer() {
    if (this.data.currentPlayingIndex >= this.data.largeBuffer.length) {
      console.log('大缓冲区播放完成')
      this.finishPlayingFromLargeBuffer()
      return
    }
    
    const audioItem = this.data.largeBuffer[this.data.currentPlayingIndex]
    
    // 检查是否是当前对话轮次的音频
    if (audioItem.dialogRound !== this.data.currentDialogRound) {
      console.log(`跳过非当前对话轮次的音频，当前轮次: ${this.data.currentDialogRound}，音频轮次: ${audioItem.dialogRound}`)
      this.data.currentPlayingIndex++
      this.playNextFromLargeBuffer()
      return
    }
    
    console.log(`播放大缓冲区音频 ${this.data.currentPlayingIndex + 1}/${this.data.largeBuffer.length}，对话轮次: ${audioItem.dialogRound}，时长: ${audioItem.duration.toFixed(3)}秒`)
    
    try {
      // 创建音频源节点
      const audioSource = this.data.webAudioContext.createBufferSource()
      audioSource.buffer = audioItem.audioBuffer
      
      // 连接到目标节点
      audioSource.connect(this.data.webAudioContext.destination)
      
      // 设置播放结束回调
      audioSource.onended = () => {
        console.log(`大缓冲区音频 ${this.data.currentPlayingIndex + 1} 播放完成`)
        this.data.currentPlayingIndex++
        this.playNextFromLargeBuffer()
      }
      
      // 开始播放
      audioSource.start()
      
      // 保存音频上下文以便管理
      this.data.audioContexts.push(audioSource)
      
    } catch (error) {
      console.error('播放大缓冲区音频失败:', error)
      this.data.currentPlayingIndex++
      this.playNextFromLargeBuffer()
    }
  },

  // 新增：完成大缓冲区播放
  finishPlayingFromLargeBuffer() {
    console.log('大缓冲区播放完成，恢复录音')
    
    // 清理音频上下文
    this.data.audioContexts.forEach(context => {
      try {
        if (context && typeof context.stop === 'function') {
          context.stop()
        }
      } catch (error) {
        console.log('停止音频上下文失败:', error)
      }
    })
    this.data.audioContexts = []
    
    // 重置播放状态
    this.setData({ 
      isPlayingFromLargeBuffer: false,
      currentPlayingIndex: 0
    })
    
    // 重置新对话轮次标记
    this.data.isNewDialogRound = false
    
    // 检查小缓冲区是否需要合并（基于时间）
    this.checkSmallBufferTimeout()
    
    // 恢复录音
    this.resumeRecordingAfterPlayback()
  },

  // 新增：从PCM数据创建AudioBuffer
  createAudioBufferFromPCM(pcmData, sampleRate, channels) {
    try {
      const frameCount = pcmData.length / channels
      
      if (frameCount <= 0) {
        console.error('无效的帧数:', frameCount)
        return null
      }
      
      // 创建AudioBuffer
      const audioBuffer = this.data.webAudioContext.createBuffer(channels, frameCount, sampleRate)
      
      // 将16位PCM数据转换为浮点数并填充到AudioBuffer
      for (let channel = 0; channel < channels; channel++) {
        const channelData = audioBuffer.getChannelData(channel)
        for (let i = 0; i < frameCount; i++) {
          // 将16位整数转换为-1到1的浮点数
          channelData[i] = pcmData[i * channels + channel] / 32768.0
        }
      }
      
      console.log('AudioBuffer创建成功 - 声道:', channels, '采样率:', sampleRate, '时长:', audioBuffer.duration.toFixed(3), '秒')
      return audioBuffer
    } catch (error) {
      console.error('创建AudioBuffer失败:', error)
      return null
    }
  },

  // 新增：播放完成后恢复录音
  resumeRecordingAfterPlayback() {
    setTimeout(() => {
      this.setData({ 
        isPlaying: false,
        isPausingRecording: false 
      })
      this.resumeCall()
      console.log('音频播放完成，恢复录音')
    }, 300) // 稍微延迟一下确保播放完全结束
  },

  // 新增：添加用户消息
  addUserMessage(content) {
    const messageId = ++this.data.messageIdCounter
    const message = {
      id: messageId,
      role: 'user',
      content: content,
      time: this.formatTime(new Date()),
      voiceUrl: null,
      isPlaying: false
    }
    
    // 移除流式消息，添加最终消息
    const messages = this.data.messages.filter(msg => msg.id !== 'streaming')
    messages.push(message)
    
    this.setData({
      messages: messages,
      scrollToView: `msg-${messageId}`
    })
  },

  // 新增：添加AI消息
  addAIMessage(content, voiceUrl = null, voiceUrls = []) {
    const messageId = ++this.data.messageIdCounter
    const message = {
      id: messageId,
      role: 'ai', // 修改为ai，与数据库表结构一致
      content: content,
      time: this.formatTime(new Date()),
      voiceUrl: voiceUrl,
      voiceUrls: voiceUrls, // 新增
      isPlaying: false
    }
    
    const messages = [...this.data.messages, message]
    this.setData({
      messages: messages,
      scrollToView: `msg-${messageId}`
    })
    
    return messageId
  },

  // 新增：顺序播放一组URL
  playVoiceUrls(urls, startIndex = 0) {
    if (!urls || urls.length === 0 || startIndex >= urls.length) {
      this.setData({ isPlaying: false, currentAudioContext: null, isPausingRecording: false })
      // 播放结束后恢复录音
      this.resumeCall()
      return
    }
    this.setData({ isPlaying: true, isPausingRecording: true })
    // 播放前暂停录音
    this.pauseCall()
    const audioContext = wx.createInnerAudioContext()
    this.setData({ currentAudioContext: audioContext })
    audioContext.src = urls[startIndex]
    audioContext.autoplay = true
    audioContext.onEnded(() => {
      audioContext.destroy()
      this.setData({ currentAudioContext: null })
      this.playVoiceUrls(urls, startIndex + 1)
    })
    audioContext.onError(() => {
      audioContext.destroy()
      this.setData({ currentAudioContext: null })
      this.playVoiceUrls(urls, startIndex + 1)
    })
  },

  // 新增：播放语音消息，支持顺序播放voiceUrls
  playVoiceMessage(e) {
    const { messageId } = e.currentTarget.dataset
    const message = this.data.messages.find(msg => msg.id == messageId)
    
    if (message && message.voiceUrls && message.voiceUrls.length > 0) {
      // 停止当前播放
      this.stopAllAudio()
      
      // 过滤掉空的URL
      const validUrls = message.voiceUrls.filter(url => url)
      
      if (validUrls.length > 0) {
        console.log('用户点击播放，开始顺序播放所有语音片段')
        this.startSequentialPlayback(validUrls, validUrls.length)
      } else {
        wx.showToast({ title: '语音不可用', icon: 'error' })
      }
    } else if (message && message.voiceUrl) {
      // 兼容单URL
      this.stopAllAudio()
      this.playVoiceUrls([message.voiceUrl])
    } else {
      wx.showToast({ title: '语音不可用', icon: 'error' })
    }
  },

  // 处理顺序语音数据
  handleSequentialVoice(speechData) {
    
    const messages = this.data.messages;
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'ai') {
        if (!messages[i].voiceUrls) {
          messages[i].voiceUrls = [];
        }
        
        // 确保数组足够长
        while (messages[i].voiceUrls.length < speechData.sequence) {
          messages[i].voiceUrls.push('');
        }
        
        // 设置对应序号的语音URL
        messages[i].voiceUrls[speechData.sequence - 1] = speechData.voice_url;
        this.setData({ messages });
        
        // 检查前3条是否都已准备好，且还未开始播放
        const MIN_PLAY_COUNT = 3;
        const firstNReady = messages[i].voiceUrls.slice(0, MIN_PLAY_COUNT).every(url => url);
        
        if (firstNReady && !this.data.playbackStarted && !this.data.shouldStartPlaying) {
          console.log(`前${MIN_PLAY_COUNT}条语音已准备好，设置播放标志`);
          this.setData({ 
            shouldStartPlaying: true,
            totalSequences: speechData.total,
            isProcessingAI: false // AI处理完成，进入播放阶段
          });
          // 启动播放检查
          this.checkAndStartPlayback(messages[i].voiceUrls, speechData.total);
        }
        break;
      }
    }
  },

  // 新增：检查并开始播放
  checkAndStartPlayback(voiceUrls, total) {
    // 检查播放标志和当前状态
    if (this.data.shouldStartPlaying && !this.data.playbackStarted && !this.data.isPlaying) {
      console.log('开始执行播放任务');
      this.setData({ 
        playbackStarted: true,
        shouldStartPlaying: false 
      });
      this.startSequentialPlayback(voiceUrls, total);
    }
  },

  // 启动顺序播放
  startSequentialPlayback(voiceUrls, total) {
    console.log(`开始顺序播放，总共${total}个语音片段`);
    this.setData({
      isPlaying: true,
      currentSequence: 0,
      totalSequences: total,
      isPausingRecording: true,
      playedUrls: new Set() // 重置已播放记录
    });
    this.pauseCall();
    this.playNextInSequence(voiceUrls, total, 1);
  },

  // 递归播放下一个语音
  playNextInSequence(voiceUrls, total, targetSequence) {
    // 检查是否被打断
    if (!this.data.isPlaying) {
      console.log('播放被打断，停止继续播放');
      this.resetPlaybackFlags();
      this.setData({ 
        isPausingRecording: false,
        isProcessingAI: false // 重置AI处理状态
      });
      this.resumeCall();
      return;
    }

    // 检查是否播放完所有片段
    if (targetSequence > total) {
      console.log('所有语音片段播放完成，等待0.5s后恢复录音');
      this.setData({ 
        isPlaying: false, 
        currentSequence: 0, 
        isPausingRecording: true, // 保持暂停状态
        isProcessingAI: false // AI处理和播放全部完成
      });
      
      // 重置播放标志
      this.resetPlaybackFlags();
      
      // 0.5s延迟后才恢复录音
      setTimeout(() => {
        if (!this.data.isPlaying) { // 确保没有新的播放开始
          this.setData({ isPausingRecording: false });
          this.resumeCall();
          console.log('延迟0.5s后恢复录音');
        }
      }, 500);
      return;
    }

    const voiceUrl = voiceUrls[targetSequence - 1];
    if (!voiceUrl) {
      console.log(`第${targetSequence}个语音还未准备好，等待中...`);
      setTimeout(() => {
        this.playNextInSequence(voiceUrls, total, targetSequence);
      }, 100);
      return;
    }

    // 检查是否已经播放过这个URL
    if (this.data.playedUrls.has(voiceUrl)) {
      console.log(`第${targetSequence}个语音已播放过，跳过`);
      // 直接播放下一个，但添加0.5s延迟
      setTimeout(() => {
        this.playNextInSequence(voiceUrls, total, targetSequence + 1);
      }, 500);
      return;
    }

    console.log(`开始播放第${targetSequence}个语音`);
    // this.stopAllAudio(); // 彻底移除递归播放时的 stopAllAudio，避免 isPlaying 被提前置为 false

    // 只需销毁上一个 audioContext
    if (this.data.currentAudioContext) {
      try {
        this.data.currentAudioContext.stop();
        this.data.currentAudioContext.destroy();
        this.setData({ currentAudioContext: null });
      } catch (e) {}
    }

    this.setData({ currentSequence: targetSequence });

    const audioContext = wx.createInnerAudioContext();
    this.setData({ currentAudioContext: audioContext });
    
    audioContext.src = voiceUrl;
    audioContext.autoplay = true;

    audioContext.onPlay(() => {
      console.log(`第${targetSequence}个语音开始播放`);
      // 标记为已播放
      const playedUrls = new Set(this.data.playedUrls);
      playedUrls.add(voiceUrl);
      this.setData({ playedUrls });
    });

    audioContext.onEnded(() => {
      console.log(`第${targetSequence}个语音播放完成`);
      audioContext.destroy();
      this.setData({ currentAudioContext: null });
      
      // 0.5s延迟后播放下一个
      setTimeout(() => {
        this.playNextInSequence(voiceUrls, total, targetSequence + 1);
      }, 500);
    });

    audioContext.onError((res) => {
      console.log(`第${targetSequence}个语音播放出错:`, res);
      audioContext.destroy();
      this.setData({ currentAudioContext: null });
      
      // 出错也要0.5s延迟后继续下一个
      setTimeout(() => {
        this.playNextInSequence(voiceUrls, total, targetSequence + 1);
      }, 500);
    });
  },

  // 新增：重置播放标志
  resetPlaybackFlags() {
    this.setData({
      shouldStartPlaying: false,
      playbackStarted: false
    });
  },

  // 新增：更新AI消息的语音URL
  updateAIMessageVoice(speechData) {
    console.log('更新AI消息语音URL:', speechData)
    
    const messages = this.data.messages
    // 找到最后一条AI消息
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'ai' && !messages[i].voiceUrl) {
        // 处理不同的数据格式
        let voiceUrl = null
        if (typeof speechData === 'string') {
          // 直接是URL字符串
          voiceUrl = speechData
        } else if (speechData && speechData.voice_url) {
          // 对象格式，包含voice_url字段
          voiceUrl = speechData.voice_url
        }
        
        if (voiceUrl) {
          messages[i].voiceUrl = voiceUrl
          this.setData({ messages: messages })
          
          console.log('设置语音URL成功:', voiceUrl)
          
          // 自动播放语音
          this.playVoiceMessage({
            currentTarget: {
              dataset: {
                voiceUrl: voiceUrl,
                messageId: messages[i].id
              }
            }
          })
        } else {
          console.error('无效的语音数据格式:', speechData)
        }
        break
      }
    }
  },

  // 新增：显示流式识别结果
  showStreamingResult(text) {
    // 创建或更新流式结果显示
    const streamingMessage = {
      id: 'streaming',
      role: 'user',
      content: text,
      time: this.formatTime(new Date()),
      isStreaming: true
    }
    
    // 更新或添加流式消息
    const messages = this.data.messages.filter(msg => msg.id !== 'streaming')
    messages.push(streamingMessage)
    
    this.setData({
      messages: messages,
      scrollToView: 'msg-streaming'
    })
  },

  // 新增：格式化时间
  formatTime(date) {
    const hours = date.getHours().toString().padStart(2, '0')
    const minutes = date.getMinutes().toString().padStart(2, '0')
    const seconds = date.getSeconds().toString().padStart(2, '0')
    return `${hours}:${minutes}:${seconds}`
  },

  // 开始录音
  startRecording() {
    const recordParams = {
      duration: 600000, // 最长10分钟
      sampleRate: 16000, // 16kHz采样率，与后端一致
      numberOfChannels: 1, // 单声道
      encodeBitRate: 96000, // 96kbps
      format: 'pcm', // 改为pcm格式
      frameSize: 1 // 1KB一帧
    }
    console.log('录音参数:', recordParams)
    this.data.recorderManager.start(recordParams)
  },

  // 停止录音
  stopRecording() {
    this.data.recorderManager.stop()
  },

  // 播放语音
  playSpeech(speechData) {
    // 检查通话状态，如果通话已结束则不播放
    if (!this.data.isCalling) {
      console.log('通话已结束，不播放语音')
      return
    }
    
    if (!speechData || !speechData.text) {
      console.log('语音数据无效，不播放')
      return
    }
    
    console.log('开始播放语音:', speechData.text)
    console.log('语音URL:', speechData.voice_url)
    
    // 设置播放状态
    this.setData({ isPlaying: true })
    
    // 如果有语音URL，直接播放
    if (speechData.voice_url && speechData.voice_url !== '') {
      console.log('使用语音URL播放:', speechData.voice_url)
      
      // 检查通话状态
      if (!this.data.isCalling) {
        console.log('通话已结束，停止播放')
        this.setData({ isPlaying: false })
        return
      }
      
      // 直接使用阿里云OSS的音频URL
      this.data.innerAudioContext.src = speechData.voice_url
      this.data.innerAudioContext.play()
      
      // 监听播放事件
      this.data.innerAudioContext.onPlay(() => {
        console.log('音频开始播放')
        // 再次检查通话状态
        if (!this.data.isCalling) {
          console.log('播放过程中通话已结束，停止播放')
          this.data.innerAudioContext.stop()
          this.setData({ isPlaying: false })
        }
      })
      
      this.data.innerAudioContext.onError((err) => {
        console.error('音频播放错误:', err)
        this.setData({ isPlaying: false })
        wx.showToast({
          title: '音频播放失败',
          icon: 'error'
        })
      })
      
      this.data.innerAudioContext.onEnded(() => {
        console.log('音频播放结束')
        this.setData({ isPlaying: false })
      })
      
      wx.showToast({
        title: `播放: ${speechData.text.substring(0, 20)}...`,
        icon: 'none',
        duration: 3000
      })
      return
    }
    
  },

  // 开始通话计时
  startCallTimer() {
    this.data.callTimer = setInterval(() => {
      this.setData({
        callDuration: this.data.callDuration + 1
      })
    }, 1000)
  },

  // 停止通话计时
  stopCallTimer() {
    if (this.data.callTimer) {
      clearInterval(this.data.callTimer)
      this.data.callTimer = null
    }
  },

  // 格式化通话时长
  formatCallDuration(seconds) {
    const minutes = Math.floor(seconds / 60)
    const remainingSeconds = seconds % 60
    return `${minutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`
  },

  // 暂停通话
  pauseCall() {
    if (this.data.isRecording) {
      this.stopRecording()
    }
    // 开始发送静音帧
    this.startSilentFrames()
  },

  // 恢复通话
  resumeCall() {
    // 停止发送静音帧
    this.stopSilentFrames()
    
    if (this.data.isCalling && !this.data.isRecording) {
      this.startRecording()
    }
  },

  // 新增：切换消息显示
  toggleMessages() {
    this.setData({
      showMessages: !this.data.showMessages
    })
    console.log('切换消息显示:', this.data.showMessages)
  },

  // 中断播放
  interruptPlayback() {
    console.log('用户主动中断播放');
    
    // 发送打断信号给后端
    if (this.data.ws && this.data.ws.readyState === 1) {
      this.data.ws.send({
        data: JSON.stringify({
          type: 'interrupt_playback',
          message: '用户主动中断音频播放'
        }),
        success: () => {
          console.log('打断信号已发送给后端');
        },
        fail: (err) => {
          console.error('发送打断信号失败:', err);
        }
      });
    }
    
    this.stopAllAudio();
    
    // 清空所有播放相关状态和列表
    this.setData({
      isPlaying: false,
      currentSequence: 0,
      totalSequences: 0,
      voiceQueue: [], // 清空语音队列
      currentAudioContext: null,
      isPausingRecording: true, // 先暂停
      playedUrls: new Set(), // 清空已播放记录
      isProcessingAI: false, // 重置AI处理状态
      hasReceivedFinal: false, // 新增：重置final结果标志
      isInterrupted: true // 设置打断状态
    });

    // 重置播放标志
    this.resetPlaybackFlags();

    // 清空当前AI消息的语音URL列表
    const messages = this.data.messages;
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'ai') {
        messages[i].voiceUrls = []; // 清空当前AI回复的语音列表
        break;
      }
    }
    this.setData({ messages });

    // 0.5s延迟后恢复录音
    setTimeout(() => {
      this.setData({ isPausingRecording: false });
      this.resumeCall();
      console.log('中断后延迟0.5s恢复录音');
    }, 500);
  },

  // 新增：停止所有音频播放
  stopAllAudio() {
    // 停止主音频播放器
    if (this.data.innerAudioContext) {
      try {
        this.data.innerAudioContext.stop()
        this.data.innerAudioContext.destroy()
        this.data.innerAudioContext = null
      } catch (error) {
        console.log('停止主音频播放器失败:', error)
      }
    }
    
    // 停止当前播放的音频上下文
    if (this.data.currentAudioContext) {
      try {
        this.data.currentAudioContext.stop()
        this.data.currentAudioContext.destroy()
        this.data.currentAudioContext = null
      } catch (error) {
        console.log('停止当前音频上下文失败:', error)
      }
    }
    
    // 停止WebAudioContext中的所有音频源
    if (this.data.webAudioContext) {
      try {
        // 停止所有音频上下文
        this.data.audioContexts.forEach(context => {
          try {
            if (context && typeof context.stop === 'function') {
              context.stop()
            }
          } catch (error) {
            console.log('停止音频上下文失败:', error)
          }
        })
        this.data.audioContexts = []
        
        // WebAudioContext的音频源会在播放完成后自动清理
        console.log('WebAudioContext音频源已清理')
      } catch (error) {
        console.log('处理WebAudioContext失败:', error)
      }
    }
    
    // 清理双缓冲区
    this.clearAudioBuffers()
    
    // 重置播放状态
    this.setData({ 
      isPlaying: false,
      currentAudioContext: null,
      currentSequence: 0,
      isPlayingFromLargeBuffer: false,
      currentPlayingIndex: 0
    })
  },

  // 新增：清理音频缓冲区
  clearAudioBuffers() {
    // 清空小缓冲区
    this.data.smallBuffer = []
    this.data.smallBufferSize = 0
    
    // 清空大缓冲区
    this.data.largeBuffer = []
    
    // 清理定时器
    if (this.data.smallBufferTimer) {
      clearTimeout(this.data.smallBufferTimer)
      this.data.smallBufferTimer = null
    }
    
    // 重置时间戳
    this.data.lastSmallBufferUpdate = 0
    
    // 重置对话轮次相关状态
    this.data.isNewDialogRound = false
    
    console.log('音频缓冲区已清理')
  },

  // 新增：生成1KB PCM静音帧
  generateSilentFrame() {
    // PCM 16bit 单声道，1KB = 1024字节 = 512个采样点
    const frameSize = 1024; // 1KB
    const buffer = new ArrayBuffer(frameSize);
    const view = new Int16Array(buffer);
    
    // 填充静音数据（全部为0）
    for (let i = 0; i < view.length; i++) {
      view[i] = 0;
    }
    
    return buffer;
  },

  // 新增：开始发送静音帧
  startSilentFrames() {
    if (this.data.silentFrameTimer) {
      return; // 已经在发送静音帧
    }
    console.log('开始发送静音帧，保持ASR连接');
    // 直接用缓存
    const silentFrame = this.data.silentFrameBuffer;
    this.data.silentFrameTimer = setInterval(() => {
      if (this.data.ws && this.data.isPausingRecording && this.data.isCalling && silentFrame) {
        try {
          // 兼容微信小程序ws实现，优先用sendSocketMessage
          if (typeof wx !== 'undefined' && wx.sendSocketMessage) {
            wx.sendSocketMessage({ data: silentFrame });
          } else if (typeof this.data.ws.send === 'function') {
            this.data.ws.send(silentFrame);
          }
          console.log('发送静音帧，大小:', silentFrame.byteLength);
        } catch (e) {
          console.error('发送静音帧失败', e);
        }
      }
    }, 100); // 每100ms发送一次
  },

  // 新增：停止发送静音帧
  stopSilentFrames() {
    if (this.data.silentFrameTimer) {
      clearInterval(this.data.silentFrameTimer);
      this.data.silentFrameTimer = null;
      console.log('停止发送静音帧');
    }
  },

  // 新增：重置final结果标志
  resetFinalFlag() {
    this.setData({ hasReceivedFinal: false })
    console.log('重置final结果标志，允许用户继续说话')
  },

  // 消息长按事件
  onMessageLongPress(e) {
    const message = e.currentTarget.dataset.message
    const role = e.currentTarget.dataset.role
    const messageId = e.currentTarget.dataset.messageId
    
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

  // 新增：删除消息
  deleteMessage(messageId) {
    wx.showModal({
      title: '确认删除',
      content: '确定要删除这条消息吗？删除后无法恢复。',
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
        if (res.statusCode === 200 && res.data.code === 200) {
          wx.showToast({
            title: '删除成功',
            icon: 'success'
          })
          // 重新加载对话以更新显示
          this.loadDialog()
        } else {
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
  },

}) 