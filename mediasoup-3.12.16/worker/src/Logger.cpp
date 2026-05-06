#define MS_CLASS "Logger"
// #define MS_LOG_DEV_LEVEL 3
#include <fstream>
#include "Logger.hpp"
#include <uv.h>

/* Class variables. */

const uint64_t Logger::pid{ static_cast<uint64_t>(uv_os_getpid()) };
thread_local Channel::ChannelSocket* Logger::channel{ nullptr };
thread_local char Logger::buffer[Logger::bufferSize];
std::ofstream Logger::logFile;
std::mutex Logger::logMutex;
std::string Logger::currentLogDateHour;
/* Class methods. */

void Logger::ClassInit(Channel::ChannelSocket* channel)
{
	//Logger::channel = channel;

	//MS_TRACE();
	CheckAndRotateLogFile();
}

void Logger::CheckAndRotateLogFile()
{
	// 获取当前时间
	auto now = std::time(nullptr);
	auto localTime = *std::localtime(&now);

	// 格式化时间为 YYYY-MM-DD-HH
	std::ostringstream oss;
	oss << std::put_time(&localTime, "%Y-%m-%d-%H");
	std::string newLogDateHour = oss.str();

	// 如果当前日志文件的时间戳与新时间戳不同，则切换日志文件
	if (newLogDateHour != currentLogDateHour)
	{
		if (logFile.is_open())
		{
			logFile.close(); // 关闭当前日志文件
		}

		// 构造新的日志文件名
		std::string logFileName = newLogDateHour + "_work.log";

		// 打开新的日志文件
		logFile.open(logFileName, std::ios::out | std::ios::app);
		if (!logFile.is_open())
		{
			std::cerr << "Failed to open log file: " << logFileName << std::endl;
		}

		// 更新当前日志文件的时间戳
		currentLogDateHour = newLogDateHour;
	}
}

void Logger::WriteLog(const char* message)
{
	std::lock_guard<std::mutex> lock(logMutex);

	// 检查是否需要切换日志文件
	CheckAndRotateLogFile();

	// 写入日志内容
	if (logFile.is_open())
	{
		logFile << message << std::endl;
		logFile.flush();
	}
}
