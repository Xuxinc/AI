App({
  globalData: {
    userInfo: null,
    token: null,
    baseUrl: 'http://172.27.40.57:9000',
    wsUrl: 'ws://172.27.40.57:9000/ws/voice-chat'
  },

  onLaunch() {
    // 设置音频忽略静音拨片，解决iOS音频播放问题
    if (wx.setInnerAudioOption) {
      wx.setInnerAudioOption({
        obeyMuteSwitch: false // 忽略静音拨片
      })
    }
    
    // 检查登录状态
    this.checkLoginStatus()
  },

  // 检查登录状态
  checkLoginStatus() {
    const token = wx.getStorageSync('token')
    const userInfo = wx.getStorageSync('userInfo')
    
    if (token && userInfo) {
      this.globalData.token = token
      this.globalData.userInfo = userInfo
    }
  },

  // 微信登录
  login() {
    return new Promise((resolve, reject) => {
      wx.login({
        success: (res) => {
          if (res.code) {
            // 发送code到后端
            wx.request({
              url: this.globalData.baseUrl + '/api/user/login',
              method: 'POST',
              data: {
                code: res.code
              },
              success: (loginRes) => {
                if (loginRes.statusCode === 200 && loginRes.data.code === 200) {
                  const data = loginRes.data.data
                  console.log('后端返回的登录数据:', data)
                  
                  this.globalData.token = data.token
                  this.globalData.userInfo = {
                    id: data.user_id,
                    nickname: data.nickname,
                    avatar: data.avatar
                  }
                  
                  console.log('设置到globalData的用户信息:', this.globalData.userInfo)
                  
                  // 保存到本地存储
                  wx.setStorageSync('token', data.token)
                  wx.setStorageSync('userInfo', this.globalData.userInfo)
                  
                  // 首次登录时提示用户是否去完善个人资料
                  if (!wx.getStorageSync('hasShownProfileTip')) {
                    wx.showModal({
                      title: '完善个人资料',
                      content: '是否现在去完善个人资料？',
                      confirmText: '去完善',
                      cancelText: '稍后再说',
                      success: (res) => {
                        if (res.confirm) {
                          wx.navigateTo({
                            url: '/pages/profile/profile'
                          })
                        }
                        wx.setStorageSync('hasShownProfileTip', true)
                      }
                    })
                  }
                  
                  resolve(data)
                } else {
                  reject(new Error(loginRes.data.message || '登录失败'))
                }
              },
              fail: reject
            })
          } else {
            reject(new Error('获取code失败'))
          }
        },
        fail: reject
      })
    })
  },

  // 检查是否已登录
  isLoggedIn() {
    return this.globalData.token !== null
  },

  // 退出登录
  logout() {
    this.globalData.token = null
    this.globalData.userInfo = null
    wx.removeStorageSync('token')
    wx.removeStorageSync('userInfo')
  }
}) 