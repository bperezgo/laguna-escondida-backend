import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";

export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const { username, password } = body;

    if (!username || !password) {
      return NextResponse.json(
        { error: "Username and password are required" },
        { status: 400 }
      );
    }

    // Call the backend signin endpoint
    const response = await fetch(`${config.apiUrl}/auth/signin`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        message: "Invalid credentials",
      }));
      return NextResponse.json(
        { error: error.message || "Authentication failed" },
        { status: response.status }
      );
    }

    const data = await response.json();
    const { token } = data;

    if (!token) {
      return NextResponse.json(
        { error: "No token received from server" },
        { status: 500 }
      );
    }

    // Create response with JWT in httpOnly, secure cookie
    const res = NextResponse.json({
      success: true,
      message: "Signed in successfully",
    });

    // Set the cookie with the JWT token
    res.cookies.set("access_token", token, {
      httpOnly: true,
      secure: process.env.NODE_ENV === "production",
      sameSite: "strict",
      maxAge: 60 * 60 * 24 * 7, // 7 days
      path: "/",
    });

    return res;
  } catch (error) {
    console.error("Signin error:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 }
    );
  }
}
