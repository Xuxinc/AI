const app = getApp()

Page({
  data: {
    userInfo: null,
    loading: false,
    editing: false,
    nickname: '',
    avatar: '',
    canIUseNicknameComp: wx.canIUse('input.type.nickname'),
    canIUseChooseAvatar: wx.canIUse('button.open-type.chooseAvatar')
  },

  onLoad() {
    this.loadUserInfo()
  },

  onShow() {
    // 页面显示时刷新用户信息
    if (app.globalData.token && !this.data.userInfo) {
      this.loadUserInfo()
    }
  },

  // 返回上一页
  navigateBack() {
    wx.navigateBack()
  },

  // 加载用户信息
  loadUserInfo() {
    if (!app.globalData.token) {
      wx.showToast({
        title: '请先登录',
        icon: 'error'
      })
      wx.navigateBack()
      return
    }

    this.setData({ loading: true })

    wx.request({
      url: app.globalData.baseUrl + '/api/user/info',
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      success: (res) => {
        if (res.statusCode === 200 && res.data.code === 200 && res.data.data && res.data.data.user) {
          const userInfo = res.data.data.user
          this.setData({
            userInfo: userInfo,
            nickname: userInfo.nickname || '',
            avatar: userInfo.avatar || ''
          })
        } else {
          wx.showToast({
            title: '获取用户信息失败',
            icon: 'error'
          })
        }
      },
      fail: () => {
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

  // 开始编辑
  startEdit() {
    this.setData({ editing: true })
  },

  // 取消编辑
  cancelEdit() {
    this.setData({
      editing: false,
      nickname: this.data.userInfo ? (this.data.userInfo.nickname || '') : '',
      avatar: this.data.userInfo ? (this.data.userInfo.avatar || '') : ''
    })
    wx.showToast({ title: '已取消编辑', icon: 'none' })
  },

  // 输入昵称（支持新的nickname组件和普通input）
  onNicknameInput(e) {
    // 新组件的值在e.detail.value中，普通input也在e.detail.value中
    const value = e.detail && e.detail.value ? e.detail.value : ''
    this.setData({
      nickname: value
    })
  },

  // 图片加载成功
  onImageLoad(e) {
    // 图片加载成功，无需特殊处理
  },

  // 图片加载失败
  onImageError(e) {
    // 如果头像加载失败，显示默认头像
    this.setData({
      avatar: ''
    })
  },



  // 选择头像（使用新的chooseAvatar组件）
  onChooseAvatar(e) {
    // 用户取消选择时，e.detail可能为null或没有avatarUrl
    if (!e.detail || !e.detail.avatarUrl) {
      return // 静默处理取消操作
    }
    
    const avatarUrl = e.detail.avatarUrl
    if (typeof avatarUrl !== 'string') {
      return
    }
    
    // 直接设置头像URL，不立即上传，等保存时再上传
    this.setData({
      avatar: avatarUrl
    })
    wx.showToast({ title: '头像已选择', icon: 'success' })
  },

  // 选择头像（本地文件，兼容旧版本）
  chooseAvatar() {
    wx.chooseImage({
      count: 1,
      sizeType: ['compressed'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        if (res.tempFilePaths && res.tempFilePaths.length > 0) {
          this.setData({
            avatar: res.tempFilePaths[0]
          })
        }
      },
      fail: () => {
        // 用户取消选择，静默处理
      }
    })
  },

  // 保存编辑
  async saveEdit() {
    if (!this.data.nickname.trim()) {
      wx.showToast({
        title: '昵称不能为空',
        icon: 'error'
      })
      return
    }
    this.setData({ loading: true })
    let avatarUrl = this.data.avatar
    
    // 检查是否需要上传头像（本地文件或HTTP临时文件）
    if (avatarUrl && (avatarUrl.startsWith('wxfile://') || avatarUrl.startsWith('http://tmp/') || avatarUrl.startsWith('http://'))) {
      try {
        wx.showLoading({ title: '上传头像中...' })
        const uploadRes = await this.uploadAvatarToServer(avatarUrl)
        const result = this.handleAvatarUploadResponse(uploadRes)
        wx.hideLoading()
        
        if (result.success) {
          avatarUrl = result.avatarUrl
        } else {
          wx.showToast({ title: result.message, icon: 'error' })
          this.setData({ loading: false })
          return
        }
      } catch (e) {
        wx.hideLoading()
        wx.showToast({ title: '头像上传失败', icon: 'error' })
        this.setData({ loading: false })
        return
      }
    }
    // 更新用户信息
    console.log('准备发送用户信息更新请求:', {
      nickname: this.data.nickname,
      avatar: avatarUrl
    })
    wx.request({
      url: app.globalData.baseUrl + '/api/user/info',
      method: 'PUT',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: {
        nickname: this.data.nickname,
        avatar: avatarUrl
      },
      success: (res) => {
        console.log('用户信息更新响应:', res)
        if (res.statusCode === 200 && res.data.code === 200 && res.data.data && res.data.data.user) {
          const updatedUser = res.data.data.user
          console.log('更新后的用户信息:', updatedUser)
          this.setData({
            userInfo: updatedUser,
            editing: false
          })
          if (app.globalData.userInfo) {
            app.globalData.userInfo.avatar = updatedUser.avatar || ''
            app.globalData.userInfo.nickname = updatedUser.nickname || ''
          }
          wx.setStorageSync('userInfo', app.globalData.userInfo)
          wx.showToast({ title: '保存成功', icon: 'success' })
        } else {
          console.error('用户信息更新失败:', res.data)
          wx.showToast({ title: '保存失败', icon: 'error' })
        }
      },
      fail: (err) => {
        console.error('用户信息更新网络错误:', err)
        wx.showToast({ title: '网络错误', icon: 'error' })
      },
      complete: () => {
        this.setData({ loading: false })
      }
    })
  },

  // 上传头像到后端
  uploadAvatarToServer(filePath) {
    return new Promise((resolve, reject) => {
      wx.uploadFile({
        url: app.globalData.baseUrl + '/api/user/upload-avatar',
        filePath: filePath,
        name: 'avatar',
        header: {
          'Authorization': 'Bearer ' + app.globalData.token
        },
        success: resolve,
        fail: reject
      })
    })
  },

  // 处理头像上传响应
  handleAvatarUploadResponse(uploadRes) {
    let uploadData = uploadRes.data
    if (typeof uploadData === 'string') {
      try {
        uploadData = JSON.parse(uploadData)
      } catch (e) {
        return { success: false, message: '头像上传失败' }
      }
    }
    
    if (uploadData && uploadData.code === 200 && uploadData.data && uploadData.data.avatar_url) {
      return { success: true, avatarUrl: uploadData.data.avatar_url }
    } else {
      return { success: false, message: '头像上传失败' }
    }
  },



  // 退出登录
  logout() {
    wx.request({
      url: app.globalData.baseUrl + '/api/user/logout',
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      success: (res) => {
        // 无论服务器响应如何，都清除本地数据
        app.logout()
        
        wx.showToast({
          title: '已退出登录',
          icon: 'success'
        })
        
        // 返回首页
        setTimeout(() => {
          wx.switchTab({
            url: '/pages/index/index'
          })
        }, 1000)
      },
      fail: () => {
        // 即使请求失败，也清除本地数据
        app.logout()
        
        wx.showToast({
          title: '已退出登录',
          icon: 'success'
        })
        
        // 返回首页
        setTimeout(() => {
          wx.switchTab({
            url: '/pages/index/index'
          })
        }, 1000)
      }
    })
  }
}) 