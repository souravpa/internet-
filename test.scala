// scala  这里使用测试脚本  路径为:/Users/track/Downloads/CampaignsSimulation.scala
package main.scala

import java.util.concurrent.TimeUnit
import io.gatling.core.Predef._
import io.gatling.http.Predef._

import scala.concurrent.duration._

class CampaignsSimulation extends Simulation {

val sce = scenario("GetCampaignsScenario")
  .repeat(2, "n") {//次数 请求次数
  exec(
    http("Get-Campaigns")
      .get("/")
      .check(status.is(200))
  )
}
val httpProtocol = http
    .baseUrl("http://localhost:8080")

setUp(sce.inject(atOnceUsers(10)).protocols(httpProtocol)) //

}