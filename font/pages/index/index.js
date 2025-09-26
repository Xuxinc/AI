const app = getApp()

const defaultAvatarUrl = 'https://mmbiz.qpic.cn/mmbiz/icTdbqWNOwNRna42FI242Lcia07jQodd2FJGIYQfG0LAJGFxM4FbnQP6yfMxBgJ0F3YRqJCJ1aPAK2dQagdusBZg/0'

Page({
  data: {
    userInfo: {
      avatarUrl: defaultAvatarUrl,
      nickName: '',
    },
    hasUserInfo: false,
    canIUseGetUserProfile: wx.canIUse('getUserProfile'),
    characters: [],
    loading: false,
    total: 0,
    showLoginModal: false,
    showSearchModal: false,
    searchName: '',
    showSearchResultModal: false,
    searchResult: {},
    showAddModal: false,
    showAddCelebrityModal: false,
    newCelebrityName: '',
    showAddCustomModal: false,
    newCustomName: '',
    newCustomDescription: '',
    newCustomAvatar: '',
    resultAvatarTall: false,
    tallCache: {}, // 按URL缓存是否为长图
    // 分类相关
    categories: [
      { key: 'all', label: '全部' },
      { key: 'public', label: '公开角色' },
      { key: 'private', label: '私密角色' }
    ],
    activeCategory: 'all',
    allCharactersRaw: [] // 原始完整数据，用于本地筛选
  },

  onLoad() {
    this.loadUserInfo()
    if (!app.globalData.token) {
      this.setData({ showLoginModal: true })
    } else {
      this.loadCharacters()
    }
  },

  onShow() {
    this.loadUserInfo()
    if (!app.globalData.token) {
      this.setData({ showLoginModal: true })
    } else {
      this.loadCharacters()
    }
  },

  // 切换分类
  onCategoryTap(e) {
    const key = e.currentTarget.dataset.key
    if (!key || key === this.data.activeCategory) return
    this.setData({ activeCategory: key })
    this.applyCategoryFilter()
  },

  // 应用分类筛选
  applyCategoryFilter() {
    const { activeCategory, allCharactersRaw, tallCache } = this.data
    const uid = (app.globalData.userInfo && app.globalData.userInfo.id) || (this.data.userInfo && this.data.userInfo.id) || null
    let list = allCharactersRaw.slice()

    if (activeCategory === 'all') {
      // 全部：不筛选
    } else if (activeCategory === 'public') {
      // 公开角色：is_created_by_user = 'no' 且 uid = 当前用户
      list = list.filter(c => c.is_created_by_user === 'no' && uid && c.uid === uid)
    } else if (activeCategory === 'private') {
      // 私密角色：is_created_by_user = 'yes' 且 uid = 当前用户
      list = list.filter(c => c.is_created_by_user === 'yes' && uid && c.uid === uid)
    }

    // 合并长图缓存
    const mapped = list.map(c => ({ ...c, isTall: tallCache[c.avatar_url] || false }))
    this.setData({ characters: mapped, total: mapped.length })
  },

  // 加载用户信息
  loadUserInfo() {
    const userInfo = wx.getStorageSync('userInfo')
    if (userInfo) {
      this.setData({ userInfo })
    }
  },

  // 加载所有名人和自定义角色
  loadCharacters() {
    if (this.data.loading) return
    this.setData({ loading: true })
    const isLoggedIn = !!app.globalData.token
    const url = isLoggedIn
      ? app.globalData.baseUrl + '/api/characters'
      : app.globalData.baseUrl + '/api/characters/public'
    const header = isLoggedIn
      ? { 'Authorization': 'Bearer ' + app.globalData.token }
      : {}
    wx.request({
      url: url,
      method: 'GET',
      header: header,
      success: (res) => {
        if (res.statusCode === 401) {
          // token无效或过期，清除本地token并弹出登录
          app.logout()
          this.setData({ showLoginModal: true })
          return
        }
        if (res.statusCode === 200) {
          const responseData = res.data
          const all = responseData.characters || []
          // 保存原始数据，用于本地筛选
          this.setData({ allCharactersRaw: all })
          // 初始分类（全部）
          this.applyCategoryFilter()
        } else {
          wx.showToast({
            title: '加载失败',
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

  // 图片加载完成后根据原始宽高判断是否为长图（高宽比>1.4即认为是长图），并缓存到tallCache
  onCharacterImageLoad(e) {
    const index = Number(e.currentTarget.dataset.index)
    const url = e.currentTarget.dataset.url
    const { width, height } = e.detail || {}
    if (Number.isNaN(index) || !width || !height) return
    const isTall = height / width > 1.4
    const key = `characters[${index}].isTall`
    const tallCache = { ...(this.data.tallCache || {}) }
    if (url) tallCache[url] = isTall
    this.setData({ [key]: isTall, tallCache })
  },

  // 下拉刷新
  onPullDownRefresh() {
    this.loadCharacters()
    wx.stopPullDownRefresh()
  },

  // 上拉加载更多（已无效，直接提示）
  onScrollToLower() {
    wx.showToast({
      title: '已经到底啦~',
      icon: 'none'
    })
  },

  // 点击名人
  onCharacterTap(e) {
    const character = e.currentTarget.dataset.character
    
    if (!this.data.userInfo) {
      this.setData({ showLoginModal: true })
      return
    }
    
    // 跳转到聊天页面，默认开启流式传输
    wx.navigateTo({
      url: `/pages/chat/chat?characterId=${character.id}&characterName=${character.name}&characterAvatar=${character.avatar_url}`
    })
  },

  // 长按名人卡片
  onCharacterLongPress(e) {
    const character = e.currentTarget.dataset.character
    
    if (!this.data.userInfo) {
      this.setData({ showLoginModal: true })
      return
    }
    
    // 根据角色类型显示不同的选项
    const itemList = ['删除角色']
    if (character.is_created_by_user === 'yes') {
      itemList.push('公开角色')
    }
    
    wx.showActionSheet({
      itemList: itemList,
      success: (res) => {
        if (res.tapIndex === 0) {
          // 删除角色
          this.deleteCharacter(character)
        } else if (res.tapIndex === 1 && character.is_created_by_user === 'yes') {
          // 公开角色
          this.publicCharacter(character)
        }
      }
    })
  },

  // 删除角色
  deleteCharacter(character) {
    wx.showModal({
      title: '确认删除',
      content: `确定要删除角色"${character.name}"吗？删除后无法恢复。`,
      confirmText: '删除',
      confirmColor: '#ff4757',
      success: (res) => {
        if (res.confirm) {
          // 在确认删除时检查权限
          if (!character.uid || character.uid != app.globalData.userInfo.id) {
            wx.showModal({
              title: '删除失败',
              content: '只能删除自己创建的角色',
              showCancel: false,
              confirmText: '确定'
            })
            return
          }
          this.performDeleteCharacter(character.id)
        }
      }
    })
  },

  // 执行删除角色
  performDeleteCharacter(characterId) {
    wx.showLoading({
      title: '删除中...'
    })

    wx.request({
      url: app.globalData.baseUrl + `/api/characters/${characterId}`,
      method: 'DELETE',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token,
        'Content-Type': 'application/json'
      },
      success: (res) => {
        wx.hideLoading()
        if (res.statusCode === 200 && res.data.code === 200) {
          wx.showToast({
            title: '删除成功',
            icon: 'success'
          })
          // 重新加载角色列表
          this.loadCharacters()
        } else {
          // 使用showModal显示完整的错误信息
          wx.showModal({
            title: '删除失败',
            content: res.data.message || '删除失败',
            showCancel: false,
            confirmText: '确定'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('删除角色失败:', err)
        wx.showModal({
          title: '网络错误',
          content: '删除角色失败，请检查网络连接后重试',
          showCancel: false,
          confirmText: '确定'
        })
      }
    })
  },

  // 公开角色
  publicCharacter(character) {
    wx.showModal({
      title: '公开角色',
      content: '公开角色需要审核以后才能被所有用户看到，确定要公开吗？',
      confirmText: '确定公开',
      confirmColor: '#007AFF',
      success: (res) => {
        if (res.confirm) {
          this.performPublicCharacter(character.id)
        }
      }
    })
  },

  // 执行公开角色
  performPublicCharacter(characterId) {
    wx.showLoading({
      title: '公开中...'
    })

    wx.request({
      url: app.globalData.baseUrl + `/api/characters/${characterId}/public`,
      method: 'POST',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token,
        'Content-Type': 'application/json'
      },
      success: (res) => {
        wx.hideLoading()
        if (res.statusCode === 200 && res.data.code === 200) {
          wx.showToast({
            title: '公开成功',
            icon: 'success'
          })
          // 重新加载角色列表
          this.loadCharacters()
        } else {
          // 使用showModal显示完整的错误信息
          wx.showModal({
            title: '公开失败',
            content: res.data.message || '公开失败',
            showCancel: false,
            confirmText: '确定'
          })
        }
      },
      fail: (err) => {
        wx.hideLoading()
        console.error('公开角色失败:', err)
        wx.showModal({
          title: '网络错误',
          content: '公开角色失败，请检查网络连接后重试',
          showCancel: false,
          confirmText: '确定'
        })
      }
    })
  },

  // 点击头像
  onAvatarTap() {
    if (!this.data.userInfo) {
      this.setData({ showLoginModal: true })
    } else {
      wx.navigateTo({
        url: '/pages/profile/profile'
      })
    }
  },

  // 搜索相关
  onSearchTap() {
    this.setData({ showSearchModal: true })
  },

  onSearchInput(e) {
    this.setData({ searchName: e.detail.value })
  },

  hideSearchModal() {
    this.setData({ 
      showSearchModal: false, 
      searchName: '' 
    })
  },

  handleSearch() {
    const name = this.data.searchName.trim()
    if (!name) {
      wx.showToast({
        title: '请输入名人姓名',
        icon: 'none'
      })
      return
    }

    wx.showLoading({ title: '搜索中...' })
    wx.request({
      url: app.globalData.baseUrl + '/api/characters/search',
      method: 'GET',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      data: { name: name },
      success: (res) => {
        if (res.statusCode === 200) {
          this.setData({
            showSearchModal: false,
            showSearchResultModal: true,
            searchResult: res.data.data,
            resultAvatarTall: false
          })
        } else {
          wx.showToast({
            title: '搜索失败',
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
        wx.hideLoading()
      }
    })
  },

  hideSearchResultModal() {
    this.setData({ 
      showSearchResultModal: false,
      searchResult: {},
      resultAvatarTall: false
    })
  },

  handleCreateFromSearch() {
    this.setData({
      showSearchResultModal: false,
      newCelebrityName: this.data.searchResult.name,
      showAddCelebrityModal: true
    })
  },

  handleGoToChat() {
    const character = this.data.searchResult.character
    this.setData({ showSearchResultModal: false })
    wx.navigateTo({
      url: `/pages/chat/chat?characterId=${character.id}&characterName=${character.name}&characterAvatar=${character.avatar_url}`
    })
  },

  // 新增相关
  onAddTap() {
    if (!this.data.userInfo) {
      this.setData({ showLoginModal: true })
      return
    }
    this.setData({ showAddModal: true })
  },

  hideAddModal() {
    this.setData({ showAddModal: false })
  },

  onAddCelebrityTap() {
    this.setData({
      showAddModal: false,
      showAddCelebrityModal: true
    })
  },

  onAddCustomTap() {
    this.setData({
      showAddModal: false,
      showAddCustomModal: true
    })
  },

  onNewCelebrityInput(e) {
    this.setData({ newCelebrityName: e.detail.value })
  },

  hideAddCelebrityModal() {
    this.setData({ 
      showAddCelebrityModal: false,
      newCelebrityName: ''
    })
  },

  handleAddCelebrity() {
    const name = this.data.newCelebrityName.trim()
    if (!name) {
      wx.showToast({
        title: '请输入名人姓名',
        icon: 'none'
      })
      return
    }
    wx.showModal({
      title: '生成提示',
      content: '头像将由AI自动生成，可能与真实人物形象存在偏差。如对头像不满意，建议选择“生成自定义角色”。是否继续生成？',
      confirmText: '继续生成',
      cancelText: '去自定义',
      success: (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '生成中...' })
          wx.request({
            url: app.globalData.baseUrl + '/api/characters/generate-celebrity',
            method: 'POST',
            header: {
              'Authorization': 'Bearer ' + app.globalData.token
            },
            data: { name: name },
            success: (res) => {
              if (res.statusCode === 200 && res.data.code === 200) {
                wx.showToast({
                  title: '生成成功',
                  icon: 'success'
                })
                this.setData({
                  showAddCelebrityModal: false,
                  newCelebrityName: ''
                })
                // 刷新首页列表
                this.loadCharacters()
                // 自动跳转到会话页面
                const character = res.data.data
                wx.navigateTo({
                  url: `/pages/chat/chat?characterId=${character.id}&characterName=${character.name}&characterAvatar=${character.avatar_url}`
                })
              } else {
                wx.showToast({
                  title: res.data.message || '生成失败',
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
              wx.hideLoading()
            }
          })
        } else if (res.cancel) {
          // 引导用户去自定义角色
          this.setData({
            showAddCelebrityModal: false,
            showAddCustomModal: true
          })
        }
      }
    })
  },

  onNewCustomNameInput(e) {
    this.setData({ newCustomName: e.detail.value })
  },

  onNewCustomDescriptionInput(e) {
    this.setData({ newCustomDescription: e.detail.value })
  },

  hideAddCustomModal() {
    this.setData({ 
      showAddCustomModal: false,
      newCustomName: '',
      newCustomDescription: '',
      newCustomAvatar: ''
    })
  },

  onChooseCustomAvatar() {
    wx.chooseImage({
      count: 1,
      sizeType: ['compressed'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        if (res.tempFilePaths && res.tempFilePaths.length > 0) {
          this.setData({ newCustomAvatar: res.tempFilePaths[0] })
        }
      },
      fail: (err) => {
        // 用户取消选择，静默处理
        const msg = (err && (err.errMsg || '')).toLowerCase()
        if (msg.includes('cancel')) return
        console.error('选择头像失败:', err)
        wx.showToast({ title: '选择失败', icon: 'error' })
      }
    })
  },

  handleAddCustom() {
    const name = this.data.newCustomName.trim()
    const description = this.data.newCustomDescription.trim()
    const avatarPath = this.data.newCustomAvatar
    if (!name) {
      wx.showToast({
        title: '请输入角色姓名',
        icon: 'none'
      })
      return
    }
    if (!description) {
      wx.showToast({
        title: '请输入角色描述',
        icon: 'none'
      })
      return
    }
    if (!avatarPath) {
      wx.showToast({
        title: '请上传头像',
        icon: 'none'
      })
      return
    }
    wx.showLoading({ title: '创建中...' })
    // 先上传头像到后端
    wx.uploadFile({
      url: app.globalData.baseUrl + '/api/characters/upload-avatar',
      filePath: avatarPath,
      name: 'avatar',
      header: {
        'Authorization': 'Bearer ' + app.globalData.token
      },
      formData: {},
      success: (uploadRes) => {
        let data = uploadRes.data
        if (typeof data === 'string') {
          try { data = JSON.parse(data) } catch (e) {}
        }
        if (data && data.code === 200 && data.data && data.data.avatar_url) {
          // 上传成功，创建角色
          wx.request({
            url: app.globalData.baseUrl + '/api/characters/generate-custom',
            method: 'POST',
            header: {
              'Authorization': 'Bearer ' + app.globalData.token
            },
            data: {
              name: name,
              description: description,
              avatar_url: data.data.avatar_url
            },
            success: (res) => {
              if (res.statusCode === 200 && res.data.code === 200) {
                wx.showToast({
                  title: '创建成功',
                  icon: 'success'
                })
                this.setData({
                  showAddCustomModal: false,
                  newCustomName: '',
                  newCustomDescription: '',
                  newCustomAvatar: ''
                })
                // 刷新首页列表
                this.loadCharacters()
                // 自动跳转到会话页面
                const character = res.data.data
                wx.navigateTo({
                  url: `/pages/chat/chat?characterId=${character.id}&characterName=${character.name}&characterAvatar=${character.avatar_url}`
                })
              } else {
                wx.showToast({
                  title: res.data.message || '创建失败',
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
              wx.hideLoading()
            }
          })
        } else {
          wx.hideLoading()
          wx.showToast({
            title: '头像上传失败',
            icon: 'error'
          })
        }
      },
      fail: () => {
        wx.hideLoading()
        wx.showToast({
          title: '头像上传失败',
          icon: 'error'
        })
      }
    })
  },

  // 登录弹窗相关
  hideLoginModal() {
    this.setData({ showLoginModal: false })
  },

  handleLogin() {
    this.setData({ showLoginModal: false })
    app.login().then(() => {
      this.loadUserInfo()
      this.loadCharacters()
    }).catch(err => {
      console.error('登录失败:', err)
      wx.showToast({
        title: '登录失败',
        icon: 'error'
      })
    })
  },

  // 移除getUserProfile方法，现在只在个人资料页提供获取微信信息功能
  onResultAvatarLoad(e) {
    const url = e.currentTarget.dataset.url
    const { width, height } = e.detail || {}
    if (!width || !height) return
    const isTall = height / width > 1.4
    const tallCache = { ...(this.data.tallCache || {}) }
    if (url) tallCache[url] = isTall
    this.setData({ resultAvatarTall: isTall, tallCache })
  }
}) 