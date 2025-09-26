const app = getApp()

Page({
  data: {
    characterId: null,
    character: null,
    isEditing: false,
    editPrompt: '',
    editVoiceModel: '',
    selectedVoiceModelIndex: 0,
    selectedVoiceModelLabel: '',
    currentVoiceModelLabel: '',
    formattedCreatedAt: '',
    formattedUpdatedAt: '',
    isAvatarTall: false, // 新增：详情头像是否为长图
    voiceModelOptions: [
      // 客服
      { value: 'longyingcui', label: '龙应催 (严肃催收男)' },
      { value: 'longyingda', label: '龙应答 (开朗高音女)' },
      { value: 'longyingjing', label: '龙应静 (低调冷静女)' },
      { value: 'longyingyan', label: '龙应严 (义正严辞女)' },
      { value: 'longyingtian', label: '龙应甜 (温柔甜美女)' },
      { value: 'longyingbing', label: '龙应冰 (尖锐强势女)' },
      { value: 'longyingtao', label: '龙应桃 (温柔淡定女)' },
      { value: 'longyingling', label: '龙应聆 (温和共情女)' },
      
      // 语音助手
      { value: 'longyumi_v2', label: 'YUMI (正经青年女)' },
      { value: 'longxiaochun_v2', label: '龙小淳 (知性积极女)' },
      { value: 'longxiaoxia_v2', label: '龙小夏 (沉稳权威女)' },
      
      // 直播
      { value: 'longanran', label: '龙安燃 (活泼质感女)' },
      { value: 'longanxuan', label: '龙安宣 (经典直播女)' },
      
      // 有声书
      { value: 'longsanshu', label: '龙三叔 (沉稳质感男)' },
      { value: 'longxiu_v2', label: '龙修 (博才说书男)' },
      { value: 'longmiao_v2', label: '龙妙 (抑扬顿挫女)' },
      { value: 'longyue_v2', label: '龙悦 (温暖磁性女)' },
      { value: 'longnan_v2', label: '龙楠 (睿智青年男)' },
      { value: 'longyuan_v2', label: '龙媛 (温暖治愈女)' },
      
      // 社交陪伴
      { value: 'longanrou', label: '龙安柔 (温柔闺蜜女)' },
      { value: 'longqiang_v2', label: '龙嫱 (浪漫风情女)' },
      { value: 'longhan_v2', label: '龙寒 (温暖痴情男)' },
      { value: 'longxing_v2', label: '龙星 (温婉邻家女)' },
      { value: 'longhua_v2', label: '龙华 (元气甜美女)' },
      { value: 'longwan_v2', label: '龙婉 (积极知性女)' },
      { value: 'longcheng_v2', label: '龙橙 (智慧青年男)' },
      { value: 'longfeifei_v2', label: '龙菲菲 (甜美娇气女)' },
      { value: 'longxiaocheng_v2', label: '龙小诚 (磁性低音男)' },
      { value: 'longzhe_v2', label: '龙哲 (呆板大暖男)' },
      { value: 'longyan_v2', label: '龙颜 (温暖春风女)' },
      { value: 'longtian_v2', label: '龙天 (磁性理智男)' },
      { value: 'longze_v2', label: '龙泽 (温暖元气男)' },
      { value: 'longshao_v2', label: '龙邵 (积极向上男)' },
      { value: 'longhao_v2', label: '龙浩 (多情忧郁男)' },
      { value: 'kabuleshen_v2', label: '龙深 (实力歌手男)' },
      
      // 童声
      { value: 'longjielidou_v2', label: '龙杰力豆 (阳光顽皮男)' },
      { value: 'longling_v2', label: '龙铃 (稚气呆板女)' },
      { value: 'longke_v2', label: '龙可 (懵懂乖乖女)' },
      { value: 'longxian_v2', label: '龙仙 (豪放可爱女)' },
      
      // 方言
      { value: 'longlaotie_v2', label: '龙老铁 (东北直率男)' },
      { value: 'longjiayi_v2', label: '龙嘉怡 (知性粤语女)' },
      { value: 'longtao_v2', label: '龙桃 (积极粤语女)' },
      
      // 诗词朗诵
      { value: 'longfei_v2', label: '龙飞 (热血磁性男)' },
      { value: 'libai_v2', label: '李白 (古代诗仙男)' },
      { value: 'longjin_v2', label: '龙津 (优雅温润男)' },
      
      // 新闻播报
      { value: 'longshu_v2', label: '龙书 (沉稳青年男)' },
      { value: 'loongbella_v2', label: 'Bella2.0 (精准干练女)' },
      { value: 'longshuo_v2', label: '龙硕 (博才干练男)' },
      { value: 'longxiaobai_v2', label: '龙小白 (沉稳播报女)' },
      { value: 'longjing_v2', label: '龙婧 (典型播音女)' },
      { value: 'loongstella_v2', label: 'loongstella (飒爽利落女)' }
    ]
  },

  onLoad(options) {
    console.log('角色详情页面加载，参数:', options)
    
    if (!options.characterId) {
      wx.showToast({
        title: '缺少角色ID',
        icon: 'error'
      })
      setTimeout(() => {
        wx.navigateBack()
      }, 1500)
      return
    }
    
    this.setData({
      characterId: parseInt(options.characterId)
    })
    
    // 设置页面标题
    wx.setNavigationBarTitle({
      title: '角色详情'
    })
    
    // 加载角色信息
    this.loadCharacterDetail()
  },

  // 详情头像加载，判断是否为长图（阈值1.4）
  onDetailAvatarLoad(e) {
    const { width, height } = e.detail || {}
    if (!width || !height) return
    const isTall = height / width > 1.4
    if (isTall !== this.data.isAvatarTall) {
      this.setData({ isAvatarTall: isTall })
    }
  },

  // 加载角色详情
  loadCharacterDetail() {
    console.log('开始加载角色详情，角色ID:', this.data.characterId)
    console.log('baseUrl:', app.globalData.baseUrl)
    
    wx.showLoading({
      title: '加载中...'
    })

    wx.request({
      url: `${app.globalData.baseUrl}/api/characters/${this.data.characterId}`,
      method: 'GET',
      header: {
        'Authorization': `Bearer ${wx.getStorageSync('token')}`
      },
      success: (res) => {
        wx.hideLoading()
        if (res.statusCode === 200) {
          const character = res.data.data
          
          // 检查是否为自定义音色（不在预设列表中的voice_id）
          let voiceModelLabel = '未知音色'
          let voiceModelIndex = this.data.voiceModelOptions.findIndex(item => item.value === character.voice_model)
          
          if (voiceModelIndex === -1 && character.voice_model) {
            // 这是自定义音色，创建显示标签
            voiceModelLabel = `${character.name} (自定义音色)`
            // 将自定义音色添加到选项列表开头
            const customVoiceOption = {
              value: character.voice_model,
              label: voiceModelLabel
            }
            this.setData({
              voiceModelOptions: [customVoiceOption, ...this.data.voiceModelOptions]
            })
            voiceModelIndex = 0 // 自定义音色现在在列表开头
          } else {
            voiceModelLabel = this.data.voiceModelOptions.find(item => item.value === character.voice_model)?.label || '未知音色'
          }
          
          // 格式化创建时间
          let formattedCreatedAt = ''
          if (character.created_at) {
            const date = new Date(character.created_at)
            formattedCreatedAt = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
          }
          
          // 格式化修改时间
          let formattedUpdatedAt = ''
          if (character.updated_at) {
            const date = new Date(character.updated_at)
            formattedUpdatedAt = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
          } else {
            formattedUpdatedAt = formattedCreatedAt // 如果没有修改时间，显示创建时间
          }
          
          this.setData({
            character: character,
            editPrompt: character.prompt,
            editVoiceModel: character.voice_model,
            selectedVoiceModelIndex: voiceModelIndex >= 0 ? voiceModelIndex : 0,
            selectedVoiceModelLabel: voiceModelLabel,
            currentVoiceModelLabel: voiceModelLabel,
            formattedCreatedAt: formattedCreatedAt,
            formattedUpdatedAt: formattedUpdatedAt
          })
        } else {
          wx.showToast({
            title: '加载失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('加载角色详情失败:', err)
        wx.showToast({
          title: '网络错误',
          icon: 'error'
        })
      }
    })
  },

  // 切换编辑模式
  toggleEdit() {
    this.setData({
      isEditing: !this.data.isEditing
    })
  },

  // 保存修改
  saveChanges() {
    if (!this.data.editPrompt.trim()) {
      wx.showToast({
        title: '提示词不能为空',
        icon: 'error'
      })
      return
    }

    // 检查音色模型是否有效
    if (!this.data.editVoiceModel) {
      wx.showToast({
        title: '请选择音色模型',
        icon: 'error'
      })
      return
    }

    const requestData = {
      prompt: this.data.editPrompt,
      voice_model: this.data.editVoiceModel
    }

    console.log('保存数据:', requestData)

    wx.showLoading({
      title: '保存中...'
    })

    wx.request({
      url: `${app.globalData.baseUrl}/api/characters/${this.data.characterId}`,
      method: 'PUT',
      header: {
        'Authorization': `Bearer ${wx.getStorageSync('token')}`,
        'Content-Type': 'application/json'
      },
      data: requestData,
      success: (res) => {
        wx.hideLoading()
        if (res.statusCode === 200) {
          wx.showToast({
            title: '保存成功',
            icon: 'success'
          })
          
          // 重新加载角色详情，确保获取最新的数据
          this.loadCharacterDetail()
          
          // 退出编辑模式
          this.setData({
            isEditing: false
          })
        } else {
          console.error('保存失败响应:', res)
          let errorMessage = '保存失败'
          if (res.data && res.data.message) {
            errorMessage = res.data.message
          }
          wx.showToast({
            title: errorMessage,
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('保存失败:', err)
        wx.showToast({
          title: '网络错误，请检查网络连接',
          icon: 'error'
        })
      }
    })
  },

  // 取消编辑
  cancelEdit() {
    const voiceModelIndex = this.data.voiceModelOptions.findIndex(item => item.value === this.data.character.voice_model)
    const voiceModelLabel = this.data.voiceModelOptions.find(item => item.value === this.data.character.voice_model)?.label || '未知音色'
    
    this.setData({
      isEditing: false,
      editPrompt: this.data.character.prompt,
      editVoiceModel: this.data.character.voice_model,
      selectedVoiceModelIndex: voiceModelIndex >= 0 ? voiceModelIndex : 0,
      selectedVoiceModelLabel: voiceModelLabel
    })
  },

  // 输入提示词
  onPromptInput(e) {
    this.setData({
      editPrompt: e.detail.value
    })
  },

  // 选择音色模型
  onVoiceModelChange(e) {
    const index = e.detail.value
    const selectedModel = this.data.voiceModelOptions[index]
    console.log('选择音色模型:', selectedModel)
    this.setData({
      editVoiceModel: selectedModel.value,
      selectedVoiceModelIndex: index,
      selectedVoiceModelLabel: selectedModel.label
    })
  },

  // 上传音色
  uploadVoice() {
    // 直接打开文件选择器
    wx.chooseMessageFile({
      count: 1,
      type: 'file',
      extension: ['mp3', 'wav', 'm4a'],
      success: (res) => {
        const tempFilePath = res.tempFiles[0].path
        const fileName = res.tempFiles[0].name
        const fileSize = res.tempFiles[0].size
        
        console.log('选择的音频文件:', res.tempFiles[0])
        
        // 检查文件大小（10MB以内）
        if (fileSize > 10 * 1024 * 1024) {
          wx.showModal({
            title: '文件过大',
            content: '音频文件不能超过10MB，请选择较小的文件。',
            showCancel: false,
            confirmText: '我知道了'
          })
          return
        }
        
        // 检查文件扩展名（兼容手机端大小写）
        const validExtensions = ['mp3', 'wav', 'm4a', 'MP3', 'WAV', 'M4A']
        const fileExtension = fileName.split('.').pop()
        if (!validExtensions.includes(fileExtension)) {
          wx.showModal({
            title: '格式不支持',
            content: `当前文件格式：${fileExtension}\n支持格式：mp3、wav、m4a`,
            showCancel: false,
            confirmText: '我知道了'
          })
          return
        }
        
        // 上传音频文件
        this.uploadAudioFile(tempFilePath)
      },
      fail: (err) => {
        console.error('选择音频文件失败:', err)
        wx.showModal({
          title: '选择失败',
          content: '选择音频文件失败，请重试。',
          showCancel: false,
          confirmText: '我知道了'
        })
      }
    })
  },

  // 上传音频文件到OSS
  uploadAudioFile(filePath) {
    wx.showLoading({
      title: '上传音频中...'
    })

    wx.uploadFile({
      url: `${app.globalData.baseUrl}/api/upload/voice`,
      filePath: filePath,
      name: 'audio',
      header: {
        'Authorization': `Bearer ${wx.getStorageSync('token')}`
      },
      success: (res) => {
        wx.hideLoading()
        const data = JSON.parse(res.data)
        
        if (data.code === 200) {
          console.log('音频上传成功:', data.data.audio_url)
          // 调用音色复刻API
          this.createVoiceModel(data.data.audio_url)
        } else {
          wx.showToast({
            title: data.message || '上传失败',
            icon: 'error'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('上传音频失败:', err)
        wx.showToast({
          title: '上传失败',
          icon: 'error'
        })
      }
    })
  },

  // 创建音色模型
  createVoiceModel(audioUrl) {
    wx.showLoading({
      title: '创建音色中...'
    })

    // 生成音色前缀（统一使用celebrity）
    const prefix = "celebrity"
    
    wx.request({
      url: `${app.globalData.baseUrl}/api/voice/create`,
      method: 'POST',
      header: {
        'Authorization': `Bearer ${wx.getStorageSync('token')}`,
        'Content-Type': 'application/json'
      },
      data: {
        audio_url: audioUrl,
        prefix: prefix,
        character_id: this.data.characterId
      },
            success: (res) => {
            wx.hideLoading()
            if (res.statusCode === 200 && res.data.code === 200) {
              wx.showToast({
                title: '音色创建成功',
                icon: 'success'
              })
              
              // 重新加载角色详情，确保获取最新的音色信息
              this.loadCharacterDetail()
            } else {
              wx.showToast({
                title: '音频文件有问题，请重新选择音频文件',
                icon: 'none'
              })
            }
          },
      fail: (err) => {
        wx.hideLoading()
        console.error('创建音色失败:', err)
        wx.showToast({
          title: '音频文件有问题，请重新选择音频文件',
          icon: 'none'
        })
      }
    })
  },

  // 预览头像
  previewAvatar() {
    if (this.data.character && this.data.character.avatar_url) {
      wx.previewImage({
        urls: [this.data.character.avatar_url],
        current: this.data.character.avatar_url
      })
    }
  }
}) 