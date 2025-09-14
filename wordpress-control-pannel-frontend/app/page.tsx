"use client"

import { useEffect, useState } from "react"
import useSWR, { mutate } from "swr"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Progress } from "@/components/ui/progress"
import {
  Bell,
  ChevronDown,
  Plus,
  Server,
  Globe,
  Activity,
  Eye,
  Trash2,
  Copy,
  ExternalLink,
  Settings,
  User,
  LogOut,
  MoreHorizontal,
  Home,
  Database,
  Shield,
  AlertTriangle,
  Loader2,
} from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { Switch } from "@/components/ui/switch"
import { toast } from "@/hooks/use-toast"
import { cn } from "@/lib/utils"

type Site = {
  projectName: string
  wpPort: number
  dbName: string
  dbPassword: string
  siteURL: string
  plugins: string[]
  status: "active" | "error" | "creating" | "down"
  adminUsername: string
  adminPassword: string
  lastChecked: string
}

type Plugin = {
  name: string;
  status: "active" | "inactive";
  version: string;
  update: "available" | "none";
  author: string;
};

type ActivityLog = {
  action: string
  timestamp: string
}

type VPSStats = {
  cpu_usage: string
  ram_usage: string
}

type ViewType = "dashboard" | "sites" | "site-detail" | "login"

const API_BASE_URL = "http://localhost:8081/api"

const fetcher = (url: string, token: string | null) =>
  fetch(url, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  }).then((res) => {
    if (!res.ok) {
      const error = new Error("An error occurred while fetching the data.")
      throw error
    }
    return res.json()
  })

export default function VPSSiteWeaver() {
  const [currentView, setCurrentView] = useState<ViewType>("login")
  const [selectedSite, setSelectedSite] = useState<Site | null>(null)
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [isDeletingSite, setIsDeletingSite] = useState(false)
  const [isRestartingSite, setIsRestartingSite] = useState(false)
  const [isCreatingBackup, setIsCreatingBackup] = useState(false)
  const [isRestoringBackup, setIsRestoringBackup] = useState(false)
  const [isLoggingIn, setIsLoggingIn] = useState(false)
  const [isLoggingOut, setIsLoggingOut] = useState(false)
  const [showPassword, setShowPassword] = useState<{ [key: string]: boolean }>({})
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
  const [token, setToken] = useState<string | null>(null)

  const { data: sites = [], error: sitesError } = useSWR<Site[]>(
    token ? [`${API_BASE_URL}/sites`, token] : null,
    ([url, token]) => fetcher(url, token),
    {
      refreshInterval: 10000, // Poll every 10 seconds
    },
  )

  const { data: activities = [], error: activitiesError } = useSWR<ActivityLog[]>(
    token ? [`${API_BASE_URL}/activities`, token] : null,
    ([url, token]) => fetcher(url, token),
    {
      refreshInterval: 10000, // Poll every 10 seconds
    },
  )

  const { data: vpsStats, error: vpsStatsError } = useSWR<VPSStats>(
    token ? [`${API_BASE_URL}/vps/stats`, token] : null,
    ([url, token]) => fetcher(url, token),
    {
      refreshInterval: 10000, // Poll every 10 seconds
    },
  )

  const { data: plugins = [], error: pluginsError } = useSWR<Plugin[]>(
    token && selectedSite ? [`${API_BASE_URL}/sites/${selectedSite.projectName}/plugins`, token] : null,
    ([url, token]) => fetcher(url, token),
    {
      refreshInterval: 10000, // Poll every 10 seconds
    },
  )

  const { data: backups = [], error: backupsError } = useSWR<string[]>(
    token && selectedSite ? [`${API_BASE_URL}/sites/${selectedSite.projectName}/backups`, token] : null,
    ([url, token]) => fetcher(url, token),
    {
      refreshInterval: 5000, // Poll every 5 seconds
    },
  )

  // Form state for creating new site
  const [newSite, setNewSite] = useState({
    name: "",
    subdomain: "",
    platform: "WordPress",
    adminUsername: "",
    adminPassword: "",
    selectedPlugins: [] as string[],
  })

  // Validation errors for new site form
  const [newSiteErrors, setNewSiteErrors] = useState({
    name: "",
    subdomain: "",
    adminUsername: "",
    adminPassword: "",
  })

  // Login form state
  const [loginForm, setLoginForm] = useState({
    username: "",
    password: "",
  })

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "active":
        return <Badge className="bg-green-100 text-green-800 hover:bg-green-100">Active</Badge>
      case "error":
        return <Badge className="bg-red-100 text-red-800 hover:bg-red-100">Error</Badge>
      case "creating":
        return <Badge className="bg-amber-100 text-amber-800 hover:bg-amber-100">Creating</Badge>
      case "down":
        return <Badge className="bg-gray-100 text-gray-800 hover:bg-gray-100">Down</Badge>
      default:
        return <Badge variant="secondary">Unknown</Badge>
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    toast({
      title: "Copied to clipboard",
      description: `${text} has been copied to your clipboard.`,
    })
  }

  const generatePassword = () => {
    const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
    let password = ""
    for (let i = 0; i < 16; i++) {
      password += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    setNewSite((prev) => ({ ...prev, adminPassword: password }))
  }

  const validateNewSiteForm = () => {
    let isValid = true
    const errors = {
      name: "",
      subdomain: "",
      adminUsername: "",
      adminPassword: "",
    }

    if (!newSite.name.trim()) {
      errors.name = "Site Name is required."
      isValid = false
    }

    if (!newSite.subdomain.trim()) {
      errors.subdomain = "Subdomain is required."
      isValid = false
    } else if (!/^[a-z0-9-]+$/.test(newSite.subdomain)) {
      errors.subdomain = "Subdomain can only contain lowercase letters, numbers, and hyphens."
      isValid = false
    }

    if (!newSite.adminUsername.trim()) {
      errors.adminUsername = "Admin Username is required."
      isValid = false
    }

    if (!newSite.adminPassword.trim()) {
      errors.adminPassword = "Admin Password is required."
      isValid = false
    } else if (newSite.adminPassword.length < 8) {
      errors.adminPassword = "Password must be at least 8 characters long."
      isValid = false
    }

    setNewSiteErrors(errors)
    return isValid
  }

  const handleCreateSite = async () => {
    if (!validateNewSiteForm()) {
      return
    }

    setIsCreating(true)
    try {
      const formData = new FormData()
      formData.append("projectName", newSite.name)
      formData.append("subdomain", newSite.subdomain)
      formData.append("platform", newSite.platform)
      formData.append("adminUsername", newSite.adminUsername)
      formData.append("adminPassword", newSite.adminPassword)
      newSite.selectedPlugins.forEach((plugin) => formData.append("selectedPlugins", plugin))

      const response = await fetch(`${API_BASE_URL}/sites`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      toast({
        title: "Website creation started",
        description: data.message,
      })
      setIsCreateModalOpen(false)
      mutate([`${API_BASE_URL}/sites`, token]) // Revalidate sites
      mutate([`${API_BASE_URL}/activities`, token]) // Revalidate activities
    } catch (error: any) {
      console.error("Error creating site:", error)
      toast({
        title: "Error",
        description: `Failed to create site: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsCreating(false)
      setNewSite({
        name: "",
        subdomain: "",
        platform: "WordPress",
        adminUsername: "",
        adminPassword: "",
        selectedPlugins: [],
      })
      setNewSiteErrors({
        name: "",
        subdomain: "",
        adminUsername: "",
        adminPassword: "",
      })
    }
  }

  const deleteSite = async (projectName: string) => {
    setIsDeletingSite(true)
    try {
      const response = await fetch(`${API_BASE_URL}/sites/${projectName}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      toast({
        title: "Website deleted",
        description: data.message,
      })
      mutate([`${API_BASE_URL}/sites`, token]) // Revalidate sites
      mutate([`${API_BASE_URL}/activities`, token]) // Revalidate activities
      setCurrentView("sites") // Go back to sites list after deletion
    } catch (error: any) {
      console.error("Error deleting site:", error)
      toast({
        title: "Error",
        description: `Failed to delete site: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsDeletingSite(false)
    }
  }

  const restartSite = async (projectName: string) => {
    setIsRestartingSite(true)
    try {
      const response = await fetch(`${API_BASE_URL}/sites/${projectName}/restart`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      toast({
        title: "Website restarted",
        description: data.message,
      })
      mutate([`${API_BASE_URL}/activities`, token]) // Revalidate activities
    } catch (error: any) {
      console.error("Error restarting site:", error)
      toast({
        title: "Error",
        description: `Failed to restart site: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsRestartingSite(false)
    }
  }

  const handleCreateBackup = async () => {
    if (!selectedSite) return
    setIsCreatingBackup(true)
    try {
      const response = await fetch(`${API_BASE_URL}/sites/${selectedSite.projectName}/backups`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.details || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      toast({
        title: "Backup Started",
        description: data.message,
      })
      mutate([`${API_BASE_URL}/sites/${selectedSite.projectName}/backups`, token]) // Revalidate backups
    } catch (error: any) {
      console.error("Error creating backup:", error)
      toast({
        title: "Error",
        description: `Failed to start backup: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsCreatingBackup(false)
    }
  }

  const handleRestoreBackup = async (backupFile: string) => {
    if (!selectedSite) return;
    if (!window.confirm(`Are you sure you want to restore this backup? This will overwrite the current site.`)) {
      return;
    }

    setIsRestoringBackup(true);
    try {
      const response = await fetch(`${API_BASE_URL}/sites/${selectedSite.projectName}/backups/restore`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ backupFile }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.details || `HTTP error! status: ${response.status}`);
      }

      const data = await response.json();
      toast({
        title: "Backup Restore Started",
        description: data.message,
      });
      mutate([`${API_BASE_URL}/activities`, token]); // Revalidate activities
    } catch (error: any) {
      console.error("Error restoring backup:", error);
      toast({
        title: "Error",
        description: `Failed to start restore: ${error.message || error}`,
        variant: "destructive",
      });
    } finally {
      setIsRestoringBackup(false);
    }
  };

  const handleLogin = async () => {
    setIsLoggingIn(true)
    try {
      const response = await fetch(`${API_BASE_URL}/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(loginForm),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      setToken(data.token)
      localStorage.setItem("jwt_token", data.token) // Store token
      setCurrentView("dashboard")
      toast({
        title: "Login Successful",
        description: data.message,
      })
    } catch (error: any) {
      console.error("Login error:", error)
      toast({
        title: "Login Failed",
        description: `Invalid username or password: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsLoggingIn(false)
    }
  }

  const handleLogout = async () => {
    setIsLoggingOut(true)
    try {
      const response = await fetch(`${API_BASE_URL}/logout`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      setToken(null)
      localStorage.removeItem("jwt_token") // Clear token
      setCurrentView("login")
      toast({
        title: "Logout Successful",
        description: data.message,
      })
    } catch (error: any) {
      console.error("Logout error:", error)
      toast({
        title: "Logout Failed",
        description: `Failed to log out: ${error.message || error}`,
        variant: "destructive",
      })
    } finally {
      setIsLoggingOut(false)
    }
  }

  const handlePluginToggle = async (pluginName: string, isActive: boolean) => {
    const action = isActive ? "deactivate" : "activate";
    try {
      const response = await fetch(`${API_BASE_URL}/sites/${selectedSite?.projectName}/plugins/${pluginName}/${action}`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data = await response.json();
      toast({
        title: `Plugin ${action}d`,
        description: data.message,
      });
      mutate([`${API_BASE_URL}/sites/${selectedSite?.projectName}/plugins`, token]); // Revalidate plugins
    } catch (error: any) {
      console.error(`Error ${action}ing plugin:`, error);
      toast({
        title: "Error",
        description: `Failed to ${action} plugin: ${error.message || error}`,
        variant: "destructive",
      });
    }
  };

  const togglePasswordVisibility = (key: string) => {
    setShowPassword((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const availablePlugins = [
    "wordpress-seo",
    "elementor",
    "woocommerce",
    "contact-form-7",
    "akismet",
    "jetpack",
    "wp-super-cache",
    "updraftplus",
  ]

  useEffect(() => {
    const storedToken = localStorage.getItem("jwt_token")
    if (storedToken) {
      setToken(storedToken)
      setCurrentView("dashboard")
    } else {
      setCurrentView("login")
    }
  }, [])

  const renderNavigation = () => (
    <div className="border-b border-slate-200 bg-white">
      <div className="flex h-16 items-center px-6">
        <div className="flex items-center space-x-4">
          <div className="flex items-center space-x-3">
            <Server className="h-8 w-8 text-indigo-600" />
            <span className="text-xl font-semibold text-slate-800">VPS Site Weaver</span>
          </div>
        </div>

        <div className="ml-auto flex items-center space-x-4">
          <div className="flex items-center space-x-2 text-sm">
            {!vpsStats && !vpsStatsError ? (
              <Loader2 className="h-4 w-4 animate-spin text-slate-400" />
            ) : vpsStatsError ? (
              <div className="h-2 w-2 rounded-full bg-red-500"></div>
            ) : (
              <div className="h-2 w-2 rounded-full bg-green-500"></div>
            )}
            <span className="text-slate-600">
              VPS:{" "}
              {!vpsStats && !vpsStatsError
                ? "Loading..."
                : vpsStatsError
                ? "Offline"
                : "Online"}
            </span>
          </div>

          <Button variant="ghost" size="icon" className="text-slate-600 hover:text-slate-800">
            <Bell className="h-5 w-5" />
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="flex items-center space-x-2 text-slate-600 hover:text-slate-800">
                <User className="h-5 w-5" />
                <span>Admin</span>
                <ChevronDown className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuItem>
                <Settings className="mr-2 h-4 w-4" />
                Settings
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleLogout} disabled={isLoggingOut}>
                {isLoggingOut && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Log out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </div>
  )

  const renderSidebar = () => (
    <div
      className={cn(
        "bg-white border-r border-slate-200 transition-all duration-200",
        sidebarCollapsed ? "w-16" : "w-64",
      )}
    >
      <nav className="p-4 space-y-2">
        <Button
          variant={currentView === "dashboard" ? "secondary" : "ghost"}
          className={cn("justify-start text-slate-600 hover:text-slate-800", sidebarCollapsed ? "w-8 px-2" : "w-full")}
          onClick={() => setCurrentView("dashboard")}
        >
          <Home className="h-4 w-4" />
          {!sidebarCollapsed && <span className="ml-2">Dashboard</span>}
        </Button>
        <Button
          variant={currentView === "sites" ? "secondary" : "ghost"}
          className={cn("justify-start text-slate-600 hover:text-slate-800", sidebarCollapsed ? "w-8 px-2" : "w-full")}
          onClick={() => setCurrentView("sites")}
        >
          <Globe className="h-4 w-4" />
          {!sidebarCollapsed && <span className="ml-2">All Websites</span>}
        </Button>
      </nav>
    </div>
  )

  const renderDashboard = () => (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-semibold text-slate-800">Dashboard</h1>
        <Dialog open={isCreateModalOpen} onOpenChange={setIsCreateModalOpen}>
          <DialogTrigger asChild>
            <Button className="bg-indigo-600 hover:bg-indigo-700 text-white">
              <Plus className="mr-2 h-4 w-4" />
              Create Website
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[600px]">
            <DialogHeader>
              <DialogTitle>Create New Website</DialogTitle>
              <DialogDescription>Set up a new containerized website on your VPS.</DialogDescription>
            </DialogHeader>

            <div className="space-y-6 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="site-name">Site Name</Label>
                  <Input
                    id="site-name"
                    placeholder="e.g., Client-X-Demo"
                    value={newSite.name}
                    onChange={(e) => setNewSite((prev) => ({ ...prev, name: e.target.value }))}
                  />
                  {newSiteErrors.name && (
                    <p className="text-red-500 text-xs mt-1">{newSiteErrors.name}</p>
                  )}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="platform">Platform</Label>
                  <Select
                    value={newSite.platform}
                    onValueChange={(value) => setNewSite((prev) => ({ ...prev, platform: value }))}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="WordPress">WordPress</SelectItem>
                      <SelectItem value="Laravel" disabled>
                        Laravel (Coming Soon)
                      </SelectItem>
                      <SelectItem value="Symfony" disabled>
                        Symfony (Coming Soon)
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="subdomain">Website URL</Label>
                <div className="flex items-center space-x-2">
                  <Input
                    id="subdomain"
                    placeholder="subdomain"
                    value={newSite.subdomain}
                    onChange={(e) => setNewSite((prev) => ({ ...prev, subdomain: e.target.value }))}
                  />
                  <span className="text-slate-500 text-sm">.myvps.com</span>
                </div>
                {newSiteErrors.subdomain && (
                  <p className="text-red-500 text-xs mt-1">{newSiteErrors.subdomain}</p>
                )}
              </div>

              <div className="border border-slate-200 rounded-lg p-4 space-y-4">
                <h4 className="font-medium text-slate-800">WordPress Administrator</h4>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="admin-username">Username</Label>
                    <Input
                      id="admin-username"
                      placeholder="admin"
                      value={newSite.adminUsername}
                      onChange={(e) => setNewSite((prev) => ({ ...prev, adminUsername: e.target.value }))}
                    />
                    {newSiteErrors.adminUsername && (
                      <p className="text-red-500 text-xs mt-1">{newSiteErrors.adminUsername}</p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="admin-password">Password</Label>
                    <div className="flex space-x-2">
                      <Input
                        id="admin-password"
                        type={showPassword["create"] ? "text" : "password"}
                        value={newSite.adminPassword}
                        onChange={(e) => setNewSite((prev) => ({ ...prev, adminPassword: e.target.value }))}
                        className="flex-1"
                      />
                      <Button type="button" variant="outline" size="sm" onClick={generatePassword}>
                        Generate
                      </Button>
                    </div>
                    {newSiteErrors.adminPassword && (
                      <p className="text-red-500 text-xs mt-1">{newSiteErrors.adminPassword}</p>
                    )}
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                <h4 className="font-medium text-slate-800">Select Plugins</h4>
                <div className="grid grid-cols-2 gap-3">
                  {availablePlugins.map((plugin) => (
                    <div key={plugin} className="flex items-center space-x-2">
                      <Checkbox
                        id={plugin}
                        checked={newSite.selectedPlugins.includes(plugin)}
                        onCheckedChange={(checked) => {
                          if (checked) {
                            setNewSite((prev) => ({
                              ...prev,
                              selectedPlugins: [...prev.selectedPlugins, plugin],
                            }))
                          } else {
                            setNewSite((prev) => ({
                              ...prev,
                              selectedPlugins: prev.selectedPlugins.filter((p) => p !== plugin),
                            }))
                          }
                        }}
                      />
                      <Label htmlFor={plugin} className="text-sm">
                        {plugin}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsCreateModalOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleCreateSite}
                disabled={isCreating || !newSite.name || !newSite.subdomain}
                className="bg-indigo-600 hover:bg-indigo-700"
              >
                {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isCreating ? "Building..." : "Build Website"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <Card className="shadow-sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-slate-600">Total Active Sites</CardTitle>
          </CardHeader>
          <CardContent>
            {sites === undefined ? (
              <div className="flex items-center justify-center h-10">
                <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
              </div>
            ) : (
              <div className="text-3xl font-semibold text-slate-800">
                {sites.filter((site) => site.status === "active").length}
              </div>
            )}
            <p className="text-sm text-slate-500 mt-1">
              {sites.filter((site) => site.status === "creating").length} creating
            </p>
          </CardContent>
        </Card>

        <Card className="shadow-sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-slate-600">Total Plugins</CardTitle>
          </CardHeader>
          <CardContent>
            {sites === undefined ? (
              <div className="flex items-center justify-center h-10">
                <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
              </div>
            ) : (
              <div className="text-3xl font-semibold text-slate-800">
                {Array.from(new Set((sites || []).flatMap((site) => site.plugins || []))).length}
              </div>
            )}
            <p className="text-sm text-slate-500 mt-1">Across all sites</p>
          </CardContent>
        </Card>

        <Card className="shadow-sm">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm font-medium text-slate-600">VPS Resources</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {vpsStats === undefined ? (
              <div className="flex items-center justify-center h-10">
                <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
              </div>
            ) : (
              <>
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-slate-600">CPU</span>
                    <span className="text-slate-800 font-medium">{vpsStats?.cpu_usage || "N/A"}%</span>
                  </div>
                  <Progress value={parseFloat(vpsStats?.cpu_usage || "0")} className="h-2" />
                </div>
                <div>
                  <div className="flex justify-between text-sm mb-1">
                    <span className="text-slate-600">RAM</span>
                    <span className="text-slate-800 font-medium">{vpsStats?.ram_usage || "N/A"}%</span>
                  </div>
                  <Progress value={parseFloat(vpsStats?.ram_usage || "0")} className="h-2" />
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent Activity */}
      <Card className="shadow-sm">
        <CardHeader>
          <CardTitle className="text-lg font-medium text-slate-800">Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          {activities === undefined ? (
            <div className="flex items-center justify-center h-20">
              <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
            </div>
          ) : (
            <div className="space-y-3">
              {activities.slice(0, 5).map((activity, index) => (
                <div
                  key={index}
                  className="flex justify-between items-center py-2 border-b border-slate-100 last:border-0"
                >
                  <span className="text-slate-700 text-sm">{activity.action}</span>
                  <span className="text-xs text-slate-500">{activity.timestamp}</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )

  const renderSitesList = () => (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-semibold text-slate-800">All Websites</h1>
        <Dialog open={isCreateModalOpen} onOpenChange={setIsCreateModalOpen}>
          <DialogTrigger asChild>
            <Button className="bg-indigo-600 hover:bg-indigo-700 text-white">
              <Plus className="mr-2 h-4 w-4" />
              Create Website
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[600px]">
            <DialogHeader>
              <DialogTitle>Create New Website</DialogTitle>
              <DialogDescription>Set up a new containerized website on your VPS.</DialogDescription>
            </DialogHeader>

            <div className="space-y-6 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="site-name">Site Name</Label>
                  <Input
                    id="site-name"
                    placeholder="e.g., Client-X-Demo"
                    value={newSite.name}
                    onChange={(e) => setNewSite((prev) => ({ ...prev, name: e.target.value }))}
                  />
                  {newSiteErrors.name && (
                    <p className="text-red-500 text-xs mt-1">{newSiteErrors.name}</p>
                  )}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="platform">Platform</Label>
                  <Select
                    value={newSite.platform}
                    onValueChange={(value) => setNewSite((prev) => ({ ...prev, platform: value }))}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="WordPress">WordPress</SelectItem>
                      <SelectItem value="Laravel" disabled>
                        Laravel (Coming Soon)
                      </SelectItem>
                      <SelectItem value="Symfony" disabled>
                        Symfony (Coming Soon)
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="subdomain">Website URL</Label>
                <div className="flex items-center space-x-2">
                  <Input
                    id="subdomain"
                    placeholder="subdomain"
                    value={newSite.subdomain}
                    onChange={(e) => setNewSite((prev) => ({ ...prev, subdomain: e.target.value }))}
                  />
                  <span className="text-slate-500 text-sm">.myvps.com</span>
                </div>
                {newSiteErrors.subdomain && (
                  <p className="text-red-500 text-xs mt-1">{newSiteErrors.subdomain}</p>
                )}
              </div>

              <div className="border border-slate-200 rounded-lg p-4 space-y-4">
                <h4 className="font-medium text-slate-800">WordPress Administrator</h4>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="admin-username">Username</Label>
                    <Input
                      id="admin-username"
                      placeholder="admin"
                      value={newSite.adminUsername}
                      onChange={(e) => setNewSite((prev) => ({ ...prev, adminUsername: e.target.value }))}
                    />
                    {newSiteErrors.adminUsername && (
                      <p className="text-red-500 text-xs mt-1">{newSiteErrors.adminUsername}</p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="admin-password">Password</Label>
                    <div className="flex space-x-2">
                      <Input
                        id="admin-password"
                        type={showPassword["create"] ? "text" : "password"}
                        value={newSite.adminPassword}
                        onChange={(e) => setNewSite((prev) => ({ ...prev, adminPassword: e.target.value }))}
                        className="flex-1"
                      />
                      <Button type="button" variant="outline" size="sm" onClick={generatePassword}>
                        Generate
                      </Button>
                    </div>
                    {newSiteErrors.adminPassword && (
                      <p className="text-red-500 text-xs mt-1">{newSiteErrors.adminPassword}</p>
                    )}
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                <h4 className="font-medium text-slate-800">Select Plugins</h4>
                <div className="grid grid-cols-2 gap-3">
                  {availablePlugins.map((plugin) => (
                    <div key={plugin} className="flex items-center space-x-2">
                      <Checkbox
                        id={plugin}
                        checked={newSite.selectedPlugins.includes(plugin)}
                        onCheckedChange={(checked) => {
                          if (checked) {
                            setNewSite((prev) => ({
                              ...prev,
                              selectedPlugins: [...prev.selectedPlugins, plugin],
                            }))
                          } else {
                            setNewSite((prev) => ({
                              ...prev,
                              selectedPlugins: prev.selectedPlugins.filter((p) => p !== plugin),
                            }))
                          }
                        }}
                      />
                      <Label htmlFor={plugin} className="text-sm">
                        {plugin}
                      </Label>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setIsCreateModalOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleCreateSite}
                disabled={isCreating || !newSite.name || !newSite.subdomain}
                className="bg-indigo-600 hover:bg-indigo-700"
              >
                {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isCreating ? "Building..." : "Build Website"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card className="shadow-sm">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="text-slate-600 font-medium">Status</TableHead>
              <TableHead className="text-slate-600 font-medium">Site Name</TableHead>
              <TableHead className="text-slate-600 font-medium">URL</TableHead>
              <TableHead className="text-slate-600 font-medium">IP Address</TableHead>
              <TableHead className="text-slate-600 font-medium">Last Checked</TableHead>
              <TableHead className="text-slate-600 font-medium w-12"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {!sites ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-8">
                  <Loader2 className="h-8 w-8 animate-spin text-slate-400 mx-auto" />
                  <p className="text-slate-500 mt-2">Loading sites...</p>
                </TableCell>
              </TableRow>
            ) : sites.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-8 text-slate-500">
                  No websites found. Create one to get started!
                </TableCell>
              </TableRow>
            ) : (
              (sites || []).map((site) => (
                <TableRow key={site.projectName} className="hover:bg-slate-50">
                  <TableCell className="py-3">{getStatusBadge(site.status)}</TableCell>
                  <TableCell className="py-3">
                    <Button
                      variant="link"
                      className="p-0 h-auto text-indigo-600 hover:text-indigo-800 font-medium"
                      onClick={() => {
                        setSelectedSite(site)
                        setCurrentView("site-detail")
                      }}
                    >
                      {site.projectName}
                    </Button>
                  </TableCell>
                  <TableCell className="py-3">
                    <div className="flex items-center space-x-2 group">
                      <span className="text-slate-700">{site.siteURL}</span>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                        onClick={() => copyToClipboard(site.siteURL)}
                      >
                        <Copy className="h-3 w-3" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                        onClick={() => window.open(`https://${site.siteURL}`, "_blank")}
                      >
                        <ExternalLink className="h-3 w-3" />
                      </Button>
                    </div>
                  </TableCell>
                  <TableCell className="py-3">
                    <div className="flex items-center space-x-2 group">
                      <span className="text-slate-700">N/A</span> {/* IP Address is not returned by backend */}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 opacity-0 group-hover:opacity-100 transition-opacity"
                        onClick={() => copyToClipboard("N/A")}
                      >
                        <Copy className="h-3 w-3" />
                      </Button>
                    </div>
                  </TableCell>
                  <TableCell className="py-3 text-slate-600">
                    {site.lastChecked ? new Date(site.lastChecked).toLocaleString() : "N/A"}
                  </TableCell>
                  <TableCell className="py-3">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8">
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={() => {
                            setSelectedSite(site)
                            setCurrentView("site-detail")
                          }}
                        >
                          <Eye className="mr-2 h-4 w-4" />
                          View Details
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          className="text-red-600"
                          onClick={() => {
                            if (
                              window.confirm(
                                `Are you sure you want to delete ${site.projectName}? Type the site name to confirm.`,
                              )
                            ) {
                              const confirmation = window.prompt(`Type "${site.projectName}" to confirm deletion:`)
                              if (confirmation === site.projectName) {
                                deleteSite(site.projectName)
                              }
                            }
                          }}
                          disabled={isDeletingSite}
                        >
                          {isDeletingSite && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  )

  const renderSiteDetail = () => {
    if (!selectedSite) return null

    return (
      <div className="p-6">
        <div className="flex items-center space-x-4 mb-6">
          <Button
            variant="ghost"
            onClick={() => setCurrentView("sites")}
            className="text-slate-600 hover:text-slate-800"
          >
            ← Back to All Websites
          </Button>
          <h1 className="text-2xl font-semibold text-slate-800">{selectedSite.projectName}</h1>
          {getStatusBadge(selectedSite.status)}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Left Column - Information Cards */}
          <div className="space-y-6">
            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="text-lg font-medium text-slate-800 flex items-center">
                  <Globe className="mr-2 h-5 w-5 text-slate-600" />
                  Key Information
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label className="text-slate-600 text-sm">Site Name</Label>
                  <p className="text-slate-800 font-medium">{selectedSite.projectName}</p>
                </div>
                <div>
                  <Label className="text-slate-600 text-sm">URL</Label>
                  <div className="flex items-center space-x-2">
                    <p className="text-slate-800 font-medium">{selectedSite.siteURL}</p>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => copyToClipboard(selectedSite.siteURL)}
                    >
                      <Copy className="h-3 w-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => window.open(`https://${selectedSite.siteURL}`, "_blank")}
                    >
                      <ExternalLink className="h-3 w-3" />
                    </Button>
                  </div>
                </div>
                <div>
                  <Label className="text-slate-600 text-sm">IP Address</Label>
                  <div className="flex items-center space-x-2">
                    <p className="text-slate-800 font-medium">N/A</p> {/* IP Address is not returned by backend */}
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => copyToClipboard("N/A")}
                    >
                      <Copy className="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>

            {selectedSite.dbPassword && ( // Using dbPassword as a proxy for admin credentials existence
              <Card className="shadow-sm">
                <CardHeader>
                  <CardTitle className="text-lg font-medium text-slate-800 flex items-center">
                    <Shield className="mr-2 h-5 w-5 text-slate-600" />
                    WordPress Credentials
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <Label className="text-slate-600 text-sm">Admin Username</Label>
                    <p className="text-slate-800 font-medium">{selectedSite.adminUsername}</p>
                  </div>
                  <div>
                    <Label className="text-slate-600 text-sm">Admin Password</Label>
                    <div className="flex items-center space-x-2">
                      <p className="text-slate-800 font-medium">
                        {showPassword[selectedSite.projectName] ? selectedSite.dbPassword : "••••••••••••"}
                      </p>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => togglePasswordVisibility(selectedSite.projectName)}
                        className="text-slate-600 hover:text-slate-800"
                      >
                        {showPassword[selectedSite.projectName] ? "Hide" : "Reveal"}
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6"
                        onClick={() => copyToClipboard(selectedSite.dbPassword || "")}
                      >
                        <Copy className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}

            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="text-lg font-medium text-slate-800 flex items-center">
                  <Database className="mr-2 h-5 w-5 text-slate-600" />
                  Database Information
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label className="text-slate-600 text-sm">Database Host</Label>
                  <p className="text-slate-800 font-medium">localhost</p>
                </div>
                <div>
                  <Label className="text-slate-600 text-sm">Database Name</Label>
                  <p className="text-slate-800 font-medium">
                    {selectedSite.dbName}
                  </p>
                </div>
                <div>
                  <Label className="text-slate-600 text-sm">Database User</Label>
                  <p className="text-slate-800 font-medium">root</p> {/* Database user is not returned by backend */}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right Column - Management & Status */}
          <div className="space-y-6">
            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="text-lg font-medium text-slate-800 flex items-center">
                  <Activity className="mr-2 h-5 w-5 text-slate-600" />
                  Container Status
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex items-center space-x-2 mb-2">{getStatusBadge(selectedSite.status)}</div>
                <p className="text-slate-500 text-sm">Last checked: {selectedSite.lastChecked ? new Date(selectedSite.lastChecked).toLocaleString() : "N/A"}</p>
              </CardContent>
            </Card>

            <Card className="shadow-sm">
              <CardHeader>
                <CardTitle className="text-lg font-medium text-slate-800">Active Plugins</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {pluginsError ? (
                    <p className="text-red-500">Failed to load plugins.</p>
                  ) : !plugins ? (
                    <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
                  ) : (
                    plugins.map((plugin, index) => (
                      <div
                        key={index}
                        className="flex items-center justify-between py-2 border-b border-slate-100 last:border-0"
                      >
                        <span className="text-slate-700">{plugin.name}</span>
                        <Switch
                          checked={plugin.status === "active"}
                          onCheckedChange={() => handlePluginToggle(plugin.name, plugin.status === "active")}
                        />
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>

            <Card className="shadow-sm">
              <CardHeader className="flex flex-row items-center justify-between pb-4">
                <CardTitle className="text-lg font-medium text-slate-800">Backups</CardTitle>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCreateBackup}
                  disabled={isCreatingBackup}
                >
                  {isCreatingBackup && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  Create New Backup
                </Button>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {!backups && !backupsError ? (
                    <Loader2 className="h-6 w-6 animate-spin text-slate-400" />
                  ) : backupsError ? (
                    <p className="text-red-500 text-sm">Failed to load backups.</p>
                  ) : backups.length === 0 ? (
                    <p className="text-slate-500 text-sm">No backups found for this site.</p>
                  ) : (
                    backups.map((backup, index) => (
                      <div
                        key={index}
                        className="flex items-center justify-between py-2 border-b border-slate-100 last:border-0"
                      >
                        <span className="text-slate-700 font-mono text-sm">{backup}</span>
                        <div>
                          <Button variant="outline" size="sm" onClick={() => handleRestoreBackup(backup)} disabled={isRestoringBackup}>
                            {isRestoringBackup && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                            Restore
                          </Button>
                          <Button variant="ghost" size="icon" className="ml-2 text-red-600 hover:text-red-700">
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>

            <Card className="shadow-sm border-red-200">
              <CardHeader>
                <CardTitle className="text-lg font-medium text-red-600 flex items-center">
                  <AlertTriangle className="mr-2 h-5 w-5" />
                  Danger Zone
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Button
                    variant="outline"
                    className="w-full border-amber-300 text-amber-700 hover:bg-amber-50 bg-transparent"
                    onClick={() => restartSite(selectedSite.projectName)}
                    disabled={isRestartingSite}
                  >
                    {isRestartingSite && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Restart Website
                  </Button>
                  <p className="text-slate-500 text-sm mt-2">Restart the Docker container for this website.</p>
                </div>
                <div>
                  <Button
                    variant="destructive"
                    className="w-full"
                    onClick={() => {
                      if (
                        window.confirm(
                          `Are you sure you want to delete ${selectedSite.projectName}? Type the site name to confirm.`,
                        )
                      ) {
                        const confirmation = window.prompt(`Type "${selectedSite.projectName}" to confirm deletion:`)
                        if (confirmation === selectedSite.projectName) {
                          deleteSite(selectedSite.projectName)
                        }
                      }
                    }}
                    disabled={isDeletingSite}
                  >
                    {isDeletingSite && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Delete Website
                  </Button>
                  <p className="text-slate-500 text-sm mt-2">Permanently delete this website and all its data.</p>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    )
  }

  const renderLogin = () => (
    <div className="flex min-h-screen items-center justify-center bg-slate-100">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl font-bold text-center">Login to VPS Site Weaver</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              placeholder="admin"
              value={loginForm.username}
              onChange={(e) => setLoginForm((prev) => ({ ...prev, username: e.target.value }))}
            />
          </div>
          <div>
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              placeholder="password"
              value={loginForm.password}
              onChange={(e) => setLoginForm((prev) => ({ ...prev, password: e.target.value }))}
            />
          </div>
          <Button onClick={handleLogin} className="w-full bg-indigo-600 hover:bg-indigo-700" disabled={isLoggingIn}>
            {isLoggingIn && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Login
          </Button>
        </CardContent>
      </Card>
    </div>
  )

  if (currentView === "login") {
    return renderLogin()
  }

  return (
    <div className="min-h-screen bg-slate-50">
      {renderNavigation()}

      <div className="flex">
        {renderSidebar()}

        <main className="flex-1">
          {currentView === "dashboard" && renderDashboard()}
          {currentView === "sites" && renderSitesList()}
          {currentView === "site-detail" && renderSiteDetail()}
        </main>
      </div>
    </div>
  )
}
