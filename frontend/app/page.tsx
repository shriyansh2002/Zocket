"use client";

import { useState, useEffect } from "react";
import axios from "axios";

export default function Home() {
  const [token, setToken] = useState<string | null>(null);
  const [tasks, setTasks] = useState<any[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [isLoadingSuggestions, setIsLoadingSuggestions] = useState(false);
  const [isRegistering, setIsRegistering] = useState(false);
  const [registerUsername, setRegisterUsername] = useState("");
  const [registerPassword, setRegisterPassword] = useState("");
  const [loginUsername, setLoginUsername] = useState("");
  const [loginPassword, setLoginPassword] = useState("");

  useEffect(() => {
    if (token) {
      console.log("Token set, fetching tasks:", token);
      fetchTasks();
      const ws = new WebSocket("ws://lucky-spontaneity-production.up.railway.app/ws/updates");
      ws.onopen = () => console.log("WebSocket connected");
      ws.onmessage = (event) => {
        const task = JSON.parse(event.data);
        console.log("New task received:", task);
        setTasks((prev) => [...prev, task]);
      };
      ws.onclose = () => console.log("WebSocket disconnected");
      return () => ws.close();
    }
  }, [token]);

  const register = async () => {
    if (!registerUsername.trim() || !registerPassword.trim()) {
      alert("Please enter both username and password");
      return;
    }
    try {
      console.log("Registering new user...");
      const res = await axios.post("https://lucky-spontaneity-production.up.railway.app/register", {
        username: registerUsername,
        password: registerPassword,
      });
      console.log("Registration response:", res.data);
      alert("Registration successful! Please login.");
      setIsRegistering(false);
      setRegisterUsername("");
      setRegisterPassword("");
    } catch (error) {
      console.error("Registration error:", error);
      alert("Registration failed. Please check console for details.");
    }
  };

  const login = async () => {
    if (!loginUsername.trim() || !loginPassword.trim()) {
      alert("Please enter both username and password");
      return;
    }
    try {
      console.log("Sending login request...");
      const res = await axios.post("https://lucky-spontaneity-production.up.railway.app/login", {
        username: loginUsername,
        password: loginPassword,
      });
      console.log("Login response:", res.data);
      if (res.data.token) {
        setToken(res.data.token);
        localStorage.setItem("token", res.data.token);
      } else {
        console.error("No token received");
      }
    } catch (error) {
      console.error("Login error:", error);
      alert("Login failed. Please check console for details.");
    }
  };

  const fetchTasks = async () => {
    try {
      const res = await axios.get("https://lucky-spontaneity-production.up.railway.app/tasks", {
        headers: { Authorization: `Bearer ${token}` },
      });
      console.log("Tasks fetched:", res.data);
      setTasks(res.data || []);
    } catch (error) {
      console.error("Fetch tasks error:", error);
      setTasks([]);
    }
  };

  const createTask = async () => {
    if (!title.trim()) {
      alert("Please enter a task title");
      return;
    }
    try {
      const res = await axios.post(
        "https://lucky-spontaneity-production.up.railway.app/tasks",
        { title, description },
        { headers: { Authorization: `Bearer ${token}` } }
      );
      console.log("Task created:", res.data);
      setTitle("");
      setDescription("");
      fetchTasks();
    } catch (error) {
      console.error("Create task error:", error);
      alert("Failed to create task");
    }
  };

  const getSuggestions = async () => {
    setIsLoadingSuggestions(true);
    try {
      console.log("Fetching suggestions...");
      const res = await axios.get("https://lucky-spontaneity-production.up.railway.app/suggest-tasks", {
        headers: { 
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json'
        }
      });
      
      console.log("Suggestions response:", res.data);
      
      if (res.data && res.data.suggestions) {
        setSuggestions(res.data.suggestions);
      } else {
        setSuggestions([]);
      }
    } catch (error) {
      console.error("Get suggestions error:", error);
      setSuggestions([]);
      alert("Failed to get suggestions");
    } finally {
      setIsLoadingSuggestions(false);
    }
  };

  const logout = () => {
    setToken(null);
    localStorage.removeItem("token");
    setTasks([]);
    setSuggestions([]);
  };

  return (
    <div className="p-4 max-w-4xl mx-auto">
      {!token ? (
        <div className="space-y-4">
          {isRegistering ? (
            // Registration Form
            <div className="bg-white p-6 rounded-lg shadow-md">
              <h2 className="text-xl font-bold mb-4">Register New User</h2>
              <div className="space-y-3">
                <input
                  type="text"
                  value={registerUsername}
                  onChange={(e) => setRegisterUsername(e.target.value)}
                  placeholder="Username"
                  className="border p-2 rounded w-full"
                />
                <input
                  type="password"
                  value={registerPassword}
                  onChange={(e) => setRegisterPassword(e.target.value)}
                  placeholder="Password"
                  className="border p-2 rounded w-full"
                />
                <div className="flex gap-2">
                  <button
                    onClick={register}
                    className="bg-green-500 text-white p-2 rounded hover:bg-green-600"
                  >
                    Register
                  </button>
                  <button
                    onClick={() => setIsRegistering(false)}
                    className="bg-gray-500 text-white p-2 rounded hover:bg-gray-600"
                  >
                    Back to Login
                  </button>
                </div>
              </div>
            </div>
          ) : (
            // Login Form
            <div className="bg-white p-6 rounded-lg shadow-md">
              <h2 className="text-xl font-bold mb-4">Login</h2>
              <div className="space-y-3">
                <input
                  type="text"
                  value={loginUsername}
                  onChange={(e) => setLoginUsername(e.target.value)}
                  placeholder="Username"
                  className="border p-2 rounded w-full"
                />
                <input
                  type="password"
                  value={loginPassword}
                  onChange={(e) => setLoginPassword(e.target.value)}
                  placeholder="Password"
                  className="border p-2 rounded w-full"
                />
                <div className="flex gap-2">
                  <button
                    onClick={login}
                    className="bg-blue-500 text-white p-2 rounded hover:bg-blue-600"
                  >
                    Login
                  </button>
                  <button
                    onClick={() => setIsRegistering(true)}
                    className="bg-purple-500 text-white p-2 rounded hover:bg-purple-600"
                  >
                    Register New User
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      ) : (
        // Task Manager Interface
        <div className="space-y-6">
          <div className="flex justify-between items-center">
            <h1 className="text-2xl font-bold">Task Manager</h1>
            <button
              onClick={logout}
              className="bg-red-500 text-white p-2 rounded hover:bg-red-600"
            >
              Logout
            </button>
          </div>

          <div className="bg-white p-6 rounded-lg shadow-md space-y-4">
            <div className="flex gap-2">
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Task Title"
                className="border p-2 rounded flex-1"
              />
              <input
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Description"
                className="border p-2 rounded flex-1"
              />
              <button
                onClick={createTask}
                className="bg-green-500 text-white p-2 rounded hover:bg-green-600"
              >
                Add Task
              </button>
            </div>

            <div>
              <button
                onClick={getSuggestions}
                className="bg-purple-500 text-white p-2 rounded hover:bg-purple-600 disabled:bg-purple-300"
                disabled={isLoadingSuggestions}
              >
                {isLoadingSuggestions ? "Loading..." : "Get AI Suggestions"}
              </button>

              <div className="mt-4">
                <h2 className="text-xl font-semibold mb-2">AI Suggestions</h2>
                <ul className="list-disc pl-5 space-y-1">
                  {suggestions.length > 0 ? (
                    suggestions.map((s, i) => (
                      <li key={i} className="text-gray-700">{s}</li>
                    ))
                  ) : (
                    <li className="text-gray-500">No suggestions available</li>
                  )}
                </ul>
              </div>
            </div>

            <div>
              <h2 className="text-xl font-semibold mb-2">Your Tasks</h2>
              <div className="space-y-2">
                {tasks.length > 0 ? (
                  tasks.map((task) => (
                    <div key={task.id} className="border p-3 rounded-lg">
                      <h3 className="font-semibold">{task.title}</h3>
                      <p className="text-gray-600">{task.description}</p>
                      <p className="text-sm text-gray-500">
                        Created: {new Date(task.created_at).toLocaleString()}
                      </p>
                    </div>
                  ))
                ) : (
                  <p className="text-gray-500">No tasks available</p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}