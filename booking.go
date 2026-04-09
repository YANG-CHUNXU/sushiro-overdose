package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func cmdList() {
	printBanner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokens, ok := tryLoadConfig()
	if !ok {
		fmt.Println("暂无配置，请先运行 sushiro")
		return
	}
	settings := tokens.toSettings()
	client := NewClient(settings)

	reservations, err := client.GetReservations(ctx)
	if err != nil {
		fmt.Println("获取预约列表失败:", err)
		return
	}

	if len(reservations) == 0 {
		fmt.Println("当前无预约记录")
		return
	}

	fmt.Println("\n=== 当前预约 ===")
	for i, r := range reservations {
		status := "未知"
		if r.Status != "" {
			status = r.Status
		}
		queueDate := r.QueueDate
		if queueDate == "" {
			queueDate = "未知"
		}
		fmt.Printf("\n  %d. Ticket #%d\n", i+1, r.TicketID)
		fmt.Printf("     号码: %s\n", r.Number)
		fmt.Printf("     日期: %s\n", queueDate)
		if r.Start != "" {
			fmt.Printf("     时段: %s-%s\n", formatCompactTime(r.Start), formatCompactTime(defaultString(r.End, r.Start)))
		}
		fmt.Printf("     状态: %s\n", status)
		if r.StoreID != "" {
			info, _ := client.GetStoreInfo(ctx, r.StoreID)
			if info.Name != "" {
				fmt.Printf("     门店: %s\n", info.Name)
			}
		}
		if r.Wait > 0 {
			fmt.Printf("     等待: %d 桌\n", r.Wait)
		}
		fmt.Printf("     成人: %d / 儿童: %d / 桌型: %s\n", r.NumAdult, r.NumChild, r.TableType)
	}
	fmt.Println()
}

func cmdCancel(args []string) {
	printBanner()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(args) == 0 {
		fmt.Println("Usage: sushiro cancel <ticket_id>")
		fmt.Println("用 sushiro list 查看预约列表")
		return
	}

	ticketID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		fmt.Println("无效的 ticket_id:", args[0])
		return
	}

	tokens, ok := tryLoadConfig()
	if !ok {
		fmt.Println("暂无配置，请先运行 sushiro")
		return
	}
	settings := tokens.toSettings()
	client := NewClient(settings)

	fmt.Printf("正在取消预约 #%d...\n", ticketID)
	if err := client.CancelReservation(ctx, ticketID); err != nil {
		fmt.Println("取消失败:", err)
		return
	}

	fmt.Printf("预约 #%d 已取消\n", ticketID)
	logMessage(time.Now(), fmt.Sprintf("预约 #%d 已取消", ticketID))
}
